package main

import (
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/types"
)

func (v *vimstate) redefineDiagnostics() error {
	v.diagnosticsLock.Lock()
	filediags := make(map[span.URI][]protocol.Diagnostic)
	for k, v := range v.diagnostics {
		filediags[k] = v.Diagnostics
	}
	doWork := v.diagnosticsChanged
	v.diagnosticsChanged = false
	v.diagnosticsLock.Unlock()

	if !doWork {
		return nil
	}

	// TODO: this will become fragile at some point
	cwd := v.ParseString(v.ChannelCall("getcwd"))

	// must be non-nil
	diags := []types.Diagnostic{}

	for uri, lspDiags := range filediags {
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
				v.Logf("redefineDiagnostics: failed to read contents of %v: %v", fn, err)
				continue
			}
			// create a temp buffer
			buf = types.NewBuffer(-1, fn, byts)
		}
		// make fn relative for reporting purposes
		fn, err := filepath.Rel(cwd, fn)
		if err != nil {
			v.Logf("redefineDiagnostics: failed to call filepath.Rel(%q, %q): %v", cwd, fn, err)
			continue
		}
		for _, d := range lspDiags {
			s, err := types.PointFromPosition(buf, d.Range.Start)
			if err != nil {
				v.Logf("redefineDiagnostics: failed to resolve start position: %v", err)
				continue
			}
			e, err := types.PointFromPosition(buf, d.Range.End)
			if err != nil {
				v.Logf("redefineDiagnostics: failed to resolve end position: %v", err)
				continue
			}
			diags = append(diags, types.Diagnostic{
				Filename: fn,
				Range:    types.Range{Start: s, End: e},
				Text:     d.Message,
				Buf:      buf.Num,
				Severity: int(d.Severity),
			})
		}
	}

	sort.Slice(diags, func(i, j int) bool {
		lhs, rhs := diags[i], diags[j]
		cmp := strings.Compare(lhs.Filename, rhs.Filename)
		if cmp == 0 {
			cmp = lhs.Range.Start.Line() - rhs.Range.Start.Line()
		}
		if cmp == 0 {
			cmp = lhs.Range.Start.Col() - rhs.Range.Start.Col()
		}
		return cmp < 0
	})

	if err := v.updateQuickfix(diags); err != nil {
		return err
	}

	if v.placeSigns() {
		if err := v.redefineSigns(diags); err != nil {
			v.Logf("redefineDiagnostics: failed to place/remove signs: %v", err)
		}
	}
	return nil
}
