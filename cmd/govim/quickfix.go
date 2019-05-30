package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"sort"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/internal/span"
	"github.com/myitcv/govim/cmd/govim/types"
)

type quickfixEntry struct {
	Filename string `json:"filename"`
	Lnum     int    `json:"lnum"`
	Col      int    `json:"col"`
	Text     string `json:"text"`
}

func (v *vimstate) quickfixDiagnostics(flags govim.CommandFlags, args ...string) error {
	return v.updateQuickfix()
}

func (v *vimstate) updateQuickfix(args ...json.RawMessage) error {
	defer func() {
		v.diagnosticsChanged = false
	}()
	if !v.diagnosticsChanged {
		return nil
	}
	var fns []span.URI
	for u := range v.diagnostics {
		fns = append(fns, u)
	}
	sort.Slice(fns, func(i, j int) bool {
		return string(fns[i]) < string(fns[j])
	})

	// TODO this will become fragile at some point
	cwd := v.ParseString(v.ChannelCall("getcwd"))

	// must be non-nil
	fixes := []quickfixEntry{}

	// now update the quickfix window based on the current diagnostics
	for _, uri := range fns {
		diags := v.diagnostics[uri]
		fn, err := uri.Filename()
		if err != nil {
			v.Logf("updateQuickfix: failed to resolve filename from URI %q: %v", uri, err)
			continue
		}
		var buf *types.Buffer
		for _, b := range v.buffers {
			if b.URI() == uri {
				buf = b
			}
		}
		if buf == nil {
			byts, err := ioutil.ReadFile(fn)
			if err != nil {
				v.Logf("updateQuickfix: failed to read contents of %v: %v", fn, err)
				continue
			}
			// create a temp buffer
			buf = &types.Buffer{
				Num:      -1,
				Name:     fn,
				Contents: byts,
			}
		}
		// make fn relative for reporting purposes
		fn, err = filepath.Rel(cwd, fn)
		if err != nil {
			v.Logf("updateQuickfix: failed to call filepath.Rel(%q, %q): %v", cwd, fn, err)
			continue
		}
		for _, d := range diags {
			p, err := types.PointFromPosition(buf, d.Range.Start)
			if err != nil {
				v.Logf("updateQuickfix: failed to resolve position: %v", err)
				continue
			}
			fixes = append(fixes, quickfixEntry{
				Filename: fn,
				Lnum:     p.Line(),
				Col:      p.Col(),
				Text:     d.Message,
			})
		}
	}
	v.ChannelCall("setqflist", fixes, "r")
	return nil
}
