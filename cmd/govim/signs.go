package main

import (
	"fmt"
)

// Using a sign group creates a separate namespace for all signs placed by govim
const signGroup = "govim"

type signDef struct {
	name          string // Name of the sign
	text          string // One or two chars shown in the gutter
	textHighlight string // Highlight used
}

// Only one sign type at the moment, errors.
var errorSign = signDef{"govimerr", ">>", "Error"}

// signDefine defines the govim error sign and must be called once before placing any signs
func (v *vimstate) signDefine() error {
	argDict := struct {
		Text          string `json:"text"`
		TextHighlight string `json:"texthl"`
	}{errorSign.text, errorSign.textHighlight}

	if v.ParseInt(v.ChannelCall("sign_define", errorSign.name, argDict)) != 0 {
		return fmt.Errorf("sign_define failed")
	}
	return nil
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

// signGetPlaced returns all signs placed by govim in a specific buffer
func (v *vimstate) signGetPlaced(buf int) (bufferSigns, error) {
	argDict := struct {
		Group string `json:"group"`
	}{signGroup}
	resp := v.ParseJSONArgSlice(v.ChannelCall("sign_getplaced", buf, argDict))

	if len(resp) != 1 {
		// Should never get here since sign_getplaced is called with a single buffer as argument
		return bufferSigns{}, fmt.Errorf("sign_getplaced failed")
	}

	var out bufferSigns
	v.Parse(resp[0], &out)
	return out, nil
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

// redefineSigns ensures that there is only one govim sign per buffer and line
// by calculating a difference between current state and the list of quickfix entries
func (v *vimstate) redefineSigns(fixes []quickfixEntry) error {
	type bufLine struct {
		buf  int
		line int
	}
	remove := make(map[bufLine]int) // Value is sign ID, used to unplace duplicates
	place := make(map[bufLine]int)  // Value is insert order, used to avoid sorting

	// Assume all existing signs should be removed, unless found in quickfix entry list
	for buf := range v.buffers {
		placed, _ := v.signGetPlaced(buf)
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

	// Add signs for quickfix entry lines that doesn't already have a sign, and
	// delete existing entries from the list of signs to removed
	inx := 0
	for _, f := range fixes {
		bl := bufLine{f.Buf, f.Lnum}
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

	if len(place) > 0 {
		placeList := make([]placeDict, len(place))
		// Use insert order as index to avoid sorting
		for bl, i := range place {
			placeList[i] = placeDict{
				Buffer: bl.buf,
				Group:  signGroup,
				Lnum:   bl.line,
				Name:   errorSign.name}
		}

		v.ChannelCall("sign_placelist", placeList)
	}

	// Remove signs on all lines that didn't have a corresponding quickfix entry
	if len(remove) > 0 {
		unplaceList := make([]unplaceDict, 0, len(remove))
		for bl, id := range remove {
			unplaceList = append(unplaceList, unplaceDict{Buffer: bl.buf, Group: signGroup, ID: id})
		}
		v.ChannelCall("sign_unplacelist", unplaceList)
	}
	return nil
}
