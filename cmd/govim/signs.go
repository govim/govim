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

// signPlace creates a new sign with an auto allocated ID
func (v *vimstate) signPlace(buf, line int) {
	argDict := struct {
		Lnum     int `json:"lnum,omitempty"`
		Priority int `json:"priority,omitempty"`
	}{line, 0} // TODO: add support for setting prio when there are more then one type of sign

	// By providing 0 as ID argument, sign_place will allocate a free ID within the provided group
	v.ChannelCall("sign_place", 0, signGroup, errorSign.name, buf, argDict)
}

// signUnplace removes a sign placed by govim
func (v *vimstate) signUnplace(buf, signID int) {
	argDict := struct {
		Buffer int `json:"buffer,omitempty"`
		ID     int `json:"id,omitempty"`
	}{buf, signID}
	v.ChannelCall("sign_unplace", signGroup, argDict)
}

// redefineSigns ensures that there is only one govim sign per buffer and line
// by calculating a difference between current state and the list of quickfix entries
func (v *vimstate) redefineSigns(fixes []quickfixEntry) error {
	type bufLine struct {
		buf  int
		line int
	}

	remove := make(map[bufLine]int) // Value is sign ID, used to unplace the sign

	// Assume all existing signs should be removed
	for buf := range v.buffers {
		placed, _ := v.signGetPlaced(buf)

		for _, sign := range placed.Signs {
			bl := bufLine{placed.BufNr, sign.Lnum}
			if id, exist := remove[bl]; exist {
				// As each sign isn't tracked, we might end up with several signs
				// on the same line when, for example, a line is removed.
				// By removing duplicates here we ensure that there is only one
				// sign per line.
				v.signUnplace(bl.buf, id)
				continue
			}
			remove[bl] = sign.ID
		}
	}

	// Place and remove signs using batched call to reduce latency
	v.BatchStart()

	// Add signs for quickfix entry lines that doesn't already have a sign, and
	// delete existing entries from the list of signs to removed
	for _, f := range fixes {
		bl := bufLine{f.Buf, f.Lnum}
		if _, exist := remove[bl]; exist {
			delete(remove, bl)
			continue
		}

		if bl.buf == -1 {
			continue // Don't place signs in unknown buffers
		}
		v.signPlace(bl.buf, bl.line)
	}

	// Remove signs
	for s, id := range remove {
		v.signUnplace(s.buf, id)
	}

	results := v.BatchEnd()
	for _, r := range results {
		if v.ParseInt(r) != 0 {
			return fmt.Errorf("at least one call in batch failed")
		}
	}

	return nil
}
