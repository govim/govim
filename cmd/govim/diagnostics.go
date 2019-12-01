package main

import (
	"io/ioutil"
	"sort"
	"strings"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/types"
)

// diagnostics returns the last received LSP diagnostics from gopls
// and acts as a lazy conversion mechanism. The purpose is to avoid converting
// lsp diagnostics unless they are needed by govim.
func (v *vimstate) diagnostics() []types.Diagnostic {
	v.rawDiagnosticsLock.Lock()
	if !v.diagnosticsChanged {
		v.rawDiagnosticsLock.Unlock()
		return v.diagnosticsCache
	}

	filediags := make(map[span.URI][]protocol.Diagnostic)
	for k, v := range v.rawDiagnostics {
		filediags[k] = v.Diagnostics
	}
	v.diagnosticsChanged = false
	v.rawDiagnosticsLock.Unlock()

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
			buf = types.NewBuffer(-1, fn, byts, false)
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
				Severity: types.Severity(d.Severity),
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

	v.diagnosticsCache = diags
	return v.diagnosticsCache
}

func (v *vimstate) redefineDiagnostics() error {
	diags := v.diagnostics()
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
