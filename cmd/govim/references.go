package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/span"
	"github.com/myitcv/govim/cmd/govim/types"
)

func (v *vimstate) references(flags govim.CommandFlags, args ...string) error {
	v.quickfixIsDiagnostics = false
	b, pos, err := v.cursorPos()
	if err != nil {
		return fmt.Errorf("failed to get current position: %v", err)
	}
	params := &protocol.ReferenceParams{
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: string(b.URI()),
			},
			Position: pos.ToPosition(),
		},
	}

	// TODO this will become fragile at some point
	cwd := v.ParseString(v.ChannelCall("getcwd"))

	// must be non-nil
	locs := []quickfixEntry{}

	refs, err := v.server.References(context.Background(), params)
	if err != nil {
		return fmt.Errorf("called to gopls.References failed: %v", err)
	}
	for _, ref := range refs {
		var buf *types.Buffer
		for _, b := range v.buffers {
			if b.URI() == span.URI(ref.URI) {
				buf = b
			}
		}
		fn := span.URI(ref.URI).Filename()
		v.Logf("fn: %v\n", fn)
		if buf == nil {
			byts, err := ioutil.ReadFile(fn)
			if err != nil {
				v.Logf("references: failed to read contents of %v: %v", fn, err)
				continue
			}
			// create a temp buffer
			buf = types.NewBuffer(-1, fn, byts)
		}
		// make fn relative for reporting purposes
		fn, err := filepath.Rel(cwd, fn)
		if err != nil {
			v.Logf("references: failed to call filepath.Rel(%q, %q): %v", cwd, fn, err)
			continue
		}
		p, err := types.PointFromPosition(buf, ref.Range.Start)
		if err != nil {
			v.Logf("references: failed to resolve position: %v", err)
			continue
		}
		line, err := buf.Line(p.Line())
		if err != nil {
			v.Logf("references: location invalid in buffer: %v", err)
			continue
		}
		locs = append(locs, quickfixEntry{
			Filename: fn,
			Lnum:     p.Line(),
			Col:      p.Col(),
			Text:     line,
		})
	}
	sort.Slice(locs, func(i, j int) bool {
		lhs, rhs := locs[i], locs[j]
		return lhs.Filename <= rhs.Filename && lhs.Lnum <= rhs.Lnum && lhs.Col < rhs.Col
	})
	v.ChannelCall("setqflist", locs, "r")
	v.ChannelEx("copen")
	return nil
}
