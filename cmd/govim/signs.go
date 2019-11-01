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

// getPlacedDict is the representation of arguments used in vim's sign_getplaced()
type getPlacedDict struct {
	Group string `json:"group"`
}

// bufferSigns represents a single element in the response from a sign_getplaced() call
type bufferSigns struct {
	Signs []struct {
		Lnum     int    `json:"lnum"`
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Priority int    `json:"priority"`
		Group    string `json:"group"`
	} `json:"signs"`
	BufNr int `json:"bufnr"`
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

// unplaceDict is the representation of arguments used in vim's sign_unplace() and sign_unplacelist()
type unplaceDict struct {
	Buffer int    `json:"buffer,omitempty"`
	Group  string `json:"group,omitempty"` // sign_unplacelist() only
	ID     int    `json:"id,omitempty"`
}

// redefineSigns ensures that there is only one govim sign per buffer line & priority
// by calculating a difference between current state and the list of diagnostics
func (v *vimstate) redefineSigns(fixes []types.Diagnostic) error {
	type bufLine struct {
		buf      int
		line     int
		priority int
	}

	remove := make(map[bufLine]int) // Value is sign ID, used to unplace duplicates
	place := make(map[bufLine]int)  // Value is insert order, used to avoid sorting

	// One call per buffer is needed since sign_getplaced() doesn't support getting
	// signs from all buffers within a specific sign group.
	v.BatchStart()
	for buf := range v.buffers {
		v.BatchChannelCall("sign_getplaced", buf, getPlacedDict{signGroup})
	}

	var bufs []bufferSigns
	for _, res := range v.BatchEnd() {
		var tmp []bufferSigns
		v.Parse(res, &tmp)
		bufs = append(bufs, tmp...)
	}

	// Assume all existing signs should be removed, unless found in quickfix entry list
	for _, placed := range bufs {
		for _, sign := range placed.Signs {
			bl := bufLine{placed.BufNr, sign.Lnum, sign.Priority}
			if _, exist := remove[bl]; exist {
				// As each sign isn't tracked individually, we might end up with several
				// same priority signs on the same line when, for example, a line is removed.
				// By removing duplicates here we ensure that there is only one sign per
				// line & priority.
				v.ChannelCall("sign_unplace", signGroup, unplaceDict{Buffer: bl.buf, ID: sign.ID})
				continue
			}
			remove[bl] = sign.ID
		}
	}

	if v.config.QuickfixSigns != nil && !*v.config.QuickfixSigns {
		return nil
	}

	// Add signs for quickfix entry lines that doesn't already have a sign, and
	// delete existing entries from the list of signs to removed
	inx := 0
	for _, f := range fixes {
		priority, ok := signPriority[f.Severity]
		if !ok {
			v.Logf("no sign priority defined for severity: %v", f.Severity)
			continue
		}
		bl := bufLine{f.Buf, f.Range.Start.Line(), priority}
		if _, exist := remove[bl]; exist {
			delete(remove, bl)
			continue
		}

		if bl.buf == -1 {
			continue // Don't place signs in unknown buffers
		}

		if _, exist := place[bl]; !exist {
			place[bl] = inx
			inx++
		}
	}

	v.BatchStart()
	if len(place) > 0 {
		placeList := make([]placeDict, len(place))
		// Use insert order as index to avoid sorting
		for bl, i := range place {
			name, ok := signName[bl.priority]
			if !ok {
				v.Logf("no sign defined for priority %d, can't place sign", bl.priority)
				continue
			}
			placeList[i] = placeDict{
				Buffer:   bl.buf,
				Group:    signGroup,
				Lnum:     bl.line,
				Priority: bl.priority,
				Name:     name}
		}
		v.BatchChannelCall("sign_placelist", placeList)
	}

	// Remove signs on all lines that didn't have a corresponding quickfix entry
	if len(remove) > 0 {
		unplaceList := make([]unplaceDict, 0, len(remove))
		for bl, id := range remove {
			unplaceList = append(unplaceList, unplaceDict{Buffer: bl.buf, Group: signGroup, ID: id})
		}
		v.BatchChannelCall("sign_unplacelist", unplaceList)
	}
	v.BatchEnd()

	return nil
}
