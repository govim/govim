package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/types"
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

	// TODO this will become fragile at some point
	cwd := v.ParseString(v.ChannelCall("getcwd"))

	// must be non-nil
	fixes := []quickfixEntry{}

	// now update the quickfix window based on the current diagnostics
	for uri, diags := range filediags {
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
	sort.Slice(fixes, func(i, j int) bool {
		lhs, rhs := fixes[i], fixes[j]
		cmp := strings.Compare(lhs.Filename, rhs.Filename)
		if cmp == 0 {
			cmp = lhs.Lnum - rhs.Lnum
		}
		if cmp == 0 {
			cmp = lhs.Col - rhs.Col
		}
		return cmp < 0
	})
	// Note: indexes are 1-based, hence 0 means "no index"
	newIdx := 0
	if len(v.lastQuickFixDiagnostics) > 0 {
		var want qflistWant
		v.Parse(v.ChannelExpr(`getqflist({"idx":0})`), &want)
		currFix := v.lastQuickFixDiagnostics[want.Idx-1]
		for i, f := range fixes {
			if currFix == f {
				newIdx = i + 1
				break
			}
		}
	}
	v.lastQuickFixDiagnostics = fixes
	v.BatchStart()
	v.BatchChannelCall("setqflist", fixes, "r")
	if newIdx > 0 {
		v.BatchChannelCall("setqflist", []quickfixEntry{}, "r", qflistWant{Idx: newIdx})
	}
	v.BatchEnd()

	if v.placeSigns() {
		if err := v.redefineSigns(fixes); err != nil {
			v.Logf("updateQuickfix: failed to place/remove signs: %v", err)
		}
	}
	return nil
}

type qflistWant struct {
	Idx int `json:"idx"`
}
