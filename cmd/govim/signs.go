package main

import (
	"fmt"

	"github.com/govim/govim/cmd/govim/internal/types"
)

// Using a sign group creates a separate namespace for all signs placed by govim
const signGroup = "govim"

// Name of different sign types used, only one at the moment, errors.
const (
	errorSign = "govimerr"
)

// defineDict is the representation of arguments used in vim's sign_define()
type defineDict struct {
	Text          string `json:"text"`   // One or two chars shown in the gutter
	TextHighlight string `json:"texthl"` // Highlight used
}

// signDefine defines the govim error sign and must be called once before placing any signs
func (v *vimstate) signDefine() error {
	arg := defineDict{
		Text:          ">>",
		TextHighlight: "Error",
	}

	if v.ParseInt(v.ChannelCall("sign_define", errorSign, arg)) != 0 {
		return fmt.Errorf("sign_define failed")
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
		Lnum  int    `json:"lnum"`
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Prio  int    `json:"priority"`
		Group string `json:"group"`
	} `json:"signs"`
	BufNr int `json:"bufnr"`
}

// placeDict is the representation of arguments used in vim's sign_place() and sign_placelist()
type placeDict struct {
	Buffer int    `json:"buffer"`          // sign_placelist() only
	Group  string `json:"group,omitempty"` // sign_placelist() only
	ID     int    `json:"id,omitempty"`    // sign_placelist() only
	Lnum   int    `json:"lnum,omitempty"`
	Name   string `json:"name"` // sign_placelist() only
	Prio   int    `json:"priority,omitempty"`
}

// unplaceDict is the representation of arguments used in vim's sign_unplace() and sign_unplacelist()
type unplaceDict struct {
	Buffer int    `json:"buffer,omitempty"`
	Group  string `json:"group,omitempty"` // sign_unplacelist() only
	ID     int    `json:"id,omitempty"`
}

// redefineSigns ensures that there is only one govim sign per buffer line
// by calculating a difference between current state and the list of quickfix entries
func (v *vimstate) redefineSigns(fixes []types.Diagnostic) error {
	type bufLine struct {
		buf  int
		line int
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
			bl := bufLine{placed.BufNr, sign.Lnum}
			if _, exist := remove[bl]; exist {
				// As each sign isn't tracked individually, we might end up with several
				// signs on the same line when, for example, a line is removed.
				// By removing duplicates here we ensure that there is only one
				// sign per line.
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
		bl := bufLine{f.Buf, f.Range.Start.Line()}
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
			placeList[i] = placeDict{
				Buffer: bl.buf,
				Group:  signGroup,
				Lnum:   bl.line,
				Name:   errorSign}
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
