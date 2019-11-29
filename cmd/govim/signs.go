package main

import (
	"fmt"

	"github.com/govim/govim/cmd/govim/internal/types"
)

// Using a sign group creates a separate namespace for all signs placed by govim
const signGroup = "govim"

// signSuffix is used when naming the different sign types. It is appended to each
// highlight type and passed in as name to sign_define().
const signSuffix = "Sign"

// Each sign in vim has a priority. If there are multiple signs on the same line, it
// is the one with highest priority that shows. For details see ":help sign-priority".
// The default priority in vim, if not specified, is 10.
var signPriority = map[types.Severity]int{
	types.SeverityErr:  14,
	types.SeverityWarn: 12,
	types.SeverityInfo: 10,
	types.SeverityHint: 8,
}

// signName is used to map a priority to the defined sign type (i.e. "sign name")
var signName = map[int]string{
	signPriority[types.SeverityErr]:  types.HighlightErr + signSuffix,
	signPriority[types.SeverityWarn]: types.HighlightWarn + signSuffix,
	signPriority[types.SeverityInfo]: types.HighlightInfo + signSuffix,
	signPriority[types.SeverityHint]: types.HighlightHint + signSuffix,
}

// defineDict is the representation of arguments used in vim's sign_define()
type defineDict struct {
	Text          string `json:"text"`   // One or two chars shown in the gutter
	TextHighlight string `json:"texthl"` // Highlight used
}

// signDefine defines the sign types (sign names) and must be called once before placing any signs
func (v *vimstate) signDefine() error {
	for _, hi := range []string{types.HighlightErr, types.HighlightWarn, types.HighlightInfo, types.HighlightHint} {
		arg := defineDict{
			Text:          ">>",
			TextHighlight: hi,
		}

		if v.ParseInt(v.ChannelCall("sign_define", hi+signSuffix, arg)) != 0 {
			return fmt.Errorf("sign_define failed")
		}
	}
	return nil
}

// placeDict is the representation of arguments used in vim's sign_place() and sign_placelist()
type placeDict struct {
	Buffer   int    `json:"buffer"`          // sign_placelist() only
	Group    string `json:"group,omitempty"` // sign_placelist() only
	ID       int    `json:"id,omitempty"`    // sign_placelist() only
	Lnum     int    `json:"lnum,omitempty"`
	Name     string `json:"name"` // sign_placelist() only
	Priority int    `json:"priority,omitempty"`
}

// updateSigns ensures that Vim is updated with signs corresponding to the
// diagnostics fixes.
func (v *vimstate) updateSigns(fixes []types.Diagnostic, force bool) error {
	if v.config.QuickfixSigns == nil || !*v.config.QuickfixSigns {
		return nil
	}
	v.diagnosticsChangedLock.Lock()
	work := v.diagnosticsChangedSigns
	v.diagnosticsChangedSigns = false
	v.diagnosticsChangedLock.Unlock()
	if !force && !work {
		return nil
	}

	// We do this by batching a removal of all govim signs then a placing of all
	// signs.
	//
	// Is this not incredibly inefficient? Always re-placing signs? Possibly,
	// but for now it has not proved to be a problem. And the simplicity of not
	// keeping track of sign state in govim is attractive.
	//
	// However, this may prove to be insufficient for a couple of reason:
	//
	// 1. despite batching the removal and additional we might see flickering
	// 2. the CPU/wire/memory load of placing lots of signs may become
	// noticeable
	//
	// Point 1, if it becomes an issue, might well be sovlable by having two
	// identical signgroups, and flipping between the two. i.e. place signs in
	// signGroup2, then remove all signs in signGroup1 (we current remove then
	// place).
	//
	// Point 2, if it becomes an issue, will require us to keep sign state in
	// govim.  This should not be a problem because govim will (in normal
	// operation) be the source of truth for these signs. Hence, the CPU and
	// memory cost can be borne by govim, and we minimise the wire exchange
	// govim <-> Vim.

	v.BatchStart()
	defer v.BatchCancelIfNotEnded()
	v.BatchAssertChannelCall(AssertIsZero, "sign_unplace", signGroup)
	var placeList []placeDict
	for _, f := range fixes {
		if f.Buf == -1 {
			// The diagnostic is for a file that we do not have open,
			// i.e. there is no buffer. Do no try and place a sign
			continue
		}
		priority, ok := signPriority[f.Severity]
		if !ok {
			return fmt.Errorf("no sign priority defined for severity: %v", f.Severity)
		}
		name, ok := signName[priority]
		if !ok {
			return fmt.Errorf("no sign defined for priority %d, can't place sign", priority)
		}
		placeList = append(placeList, placeDict{
			Buffer:   f.Buf,
			Group:    signGroup,
			Lnum:     f.Range.Start.Line(),
			Priority: priority,
			Name:     name})
	}
	if len(placeList) > 0 {
		v.BatchChannelCall("sign_placelist", placeList)
	}
	v.BatchEnd()

	return nil
}
