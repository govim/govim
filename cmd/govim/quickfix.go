package main

import (
	"path/filepath"

	"github.com/govim/govim"
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
	return v.redefineDiagnostics()
}

func (v *vimstate) updateQuickfix(diags []types.Diagnostic) error {
	// must be non-nil
	fixes := []quickfixEntry{}

	for _, d := range diags {
		// make fn relative for reporting purposes
		fn, err := filepath.Rel(v.workingDirectory, d.Filename)
		if err != nil {
			v.Logf("redefineDiagnostics: failed to call filepath.Rel(%q, %q): %v", v.workingDirectory, fn, err)
			continue
		}

		fixes = append(fixes, quickfixEntry{
			Filename: fn,
			Lnum:     d.Range.Start.Line(),
			Col:      d.Range.Start.Col(),
			Text:     d.Text,
			Buf:      d.Buf,
		})
	}

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

	return nil
}

type qflistWant struct {
	Idx int `json:"idx"`
}
