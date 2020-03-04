package main

import (
	"fmt"

	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/types"
)

// Using a sign group creates a separate namespace for all signs placed by govim
const signGroup = "govim"

// signName is used to map a priority to the defined sign type (i.e. "sign name")
// Note that we reuse the highlight name as sign name even if they are not the same thing.
var signName = map[int]config.Highlight{
	types.SeverityPriority[types.SeverityErr]:  config.HighlightSignErr,
	types.SeverityPriority[types.SeverityWarn]: config.HighlightSignWarn,
	types.SeverityPriority[types.SeverityInfo]: config.HighlightSignInfo,
	types.SeverityPriority[types.SeverityHint]: config.HighlightSignHint,
}

// defineDict is the representation of arguments used in vim's sign_define()
type defineDict struct {
	Text          string `json:"text"`   // One or two chars shown in the gutter
	TextHighlight string `json:"texthl"` // Highlight used
}

// signDefine defines the sign types (sign names) and must be called once before placing any signs
func (v *vimstate) signDefine() error {
	signnames := []config.Highlight{
		config.HighlightSignErr,
		config.HighlightSignWarn,
		config.HighlightSignInfo,
		config.HighlightSignHint,
	}
	var useDefault []config.Highlight

	// The user might have defined a sign name already in their vimrc, and govim should respect
	// that and not override it.
	v.BatchStart()
	for _, hi := range signnames {
		v.BatchChannelCall("sign_getdefined", string(hi))
	}
	for i, res := range v.MustBatchEnd() {
		var d []defineDict
		v.Parse(res, &d)
		if len(d) == 0 {
			useDefault = append(useDefault, signnames[i])
		}
	}

	// Define default sign names
	v.BatchStart()
	for _, hi := range useDefault {
		arg := defineDict{
			Text:          ">>",
			TextHighlight: string(hi),
		}

		v.BatchChannelCall("sign_define", hi, arg)
	}
	for _, res := range v.MustBatchEnd() {
		if v.ParseInt(res) != 0 {
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
func (v *vimstate) updateSigns(force bool) error {
	if v.config.QuickfixSigns == nil || !*v.config.QuickfixSigns {
		return nil
	}
	diagsRef := v.diagnostics()
	work := v.lastDiagnosticsSigns != diagsRef
	v.lastDiagnosticsSigns = diagsRef
	if !force && !work {
		return nil
	}
	diags := *diagsRef

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
	v.BatchAssertChannelCall(AssertIsZero(), "sign_unplace", signGroup)
	var placeList []placeDict
	for _, f := range diags {
		if f.Buf == -1 {
			// The diagnostic is for a file that we do not have open,
			// i.e. there is no buffer. Do no try and place a sign
			continue
		}
		priority, ok := types.SeverityPriority[f.Severity]
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
			Name:     string(name)})
	}
	if len(placeList) > 0 {
		// Suppress E158 "Invalid buffer name" when placing signs since we might, in a rare race
		// case, try to place signs into a buffer that was just closed. Note that vim already accept
		// sign_placelist() calls with line numbers outside the buffer without throwing any error.
		v.BatchAssertChannelCall(AssertIsErrorOrNil("^Vim(let):E158:"), "sign_placelist", placeList)
	}
	v.MustBatchEnd()

	return nil
}
