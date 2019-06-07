package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"sort"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/span"
	"github.com/myitcv/govim/cmd/govim/types"
)

type quickfixEntry struct {
	Filename string `json:"filename"`
	Lnum     int    `json:"lnum"`
	Col      int    `json:"col"`
	Text     string `json:"text"`
	Buf      int    `json:"buf"`
}

func (v *vimstate) quickfixDiagnostics(flags govim.CommandFlags, args ...string) error {
	v.diagnosticsChanged = true
	v.quickfixIsDiagnostics = true
	return v.updateQuickfix()
}

func (v *vimstate) updateQuickfix(args ...json.RawMessage) error {
	v.diagnosticsLock.Lock()
	diags := make(map[span.URI][]protocol.Diagnostic)
	for k, v := range v.diagnostics {
		diags[k] = v
	}
	doWork := v.diagnosticsChanged
	v.diagnosticsChanged = false
	v.diagnosticsLock.Unlock()

	if !doWork {
		return nil
	}

	var fns []span.URI
	for u := range diags {
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
		diags := diags[uri]
		fn := uri.Filename()
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
			buf = types.NewBuffer(-1, fn, byts)
		}
		// make fn relative for reporting purposes
		fn, err := filepath.Rel(cwd, fn)
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
				Buf:      buf.Num,
			})
		}
	}
	v.ChannelCall("setqflist", fixes, "r")

	if err := v.redefineSigns(fixes); err != nil {
		v.Logf("updateQuickfix: failed to place/remove signs: %v", err)
	}
	return nil
}
