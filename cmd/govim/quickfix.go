package main

import (
	"path"
	"path/filepath"

	"github.com/govim/govim"
)

const (
	quickfixDiagnosticsTitle = "govim diagnostics"
)

type quickfixEntry struct {
	Filename string `json:"filename"`
	Lnum     int    `json:"lnum"`
	Col      int    `json:"col"`
	Text     string `json:"text"`
	Buf      int    `json:"buf"`
}

func (q quickfixEntry) equalModuloBuffer(q2 quickfixEntry) bool {
	lhs := q
	lhs.Buf = 0
	rhs := q2
	rhs.Buf = 0
	return lhs == rhs
}

func (v *vimstate) quickfixDiagnostics(flags govim.CommandFlags, args ...string) error {
	return v.updateQuickfixWithDiagnostics(true)
}

// updateQuickfixWithDiagnostics updates Vim's quickfix window with the current
// diagnostics(), respecting config settings that are overridden by force.
func (v *vimstate) updateQuickfixWithDiagnostics(force bool) error {
	diags := v.diagnostics()
	diagsHasChanged := v.lastDiagnosticsQuickfix != diags
	canDiagnostics := v.quickfixCanDiagnostics()
	autoDiag := v.config.QuickfixAutoDiagnostics == nil || *v.config.QuickfixAutoDiagnostics
	v.lastDiagnosticsQuickfix = diags
	if !force && (!diagsHasChanged || !canDiagnostics || !autoDiag) {
		return nil
	}

	// must be non-nil
	fixes := []quickfixEntry{}
	for _, d := range *diags {
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
	//
	// If we were previously not showing diagnostics, we default to selection
	// the first entry. In the future we might want to improve this logic
	// by stashing the last selected diagnostic when we flip to, for example,
	// references mode. But for now we keep it simple.
	newIdx := 0
	if canDiagnostics && len(v.lastQuickFixDiagnostics) > 0 {
		var qflist qflistProps
		v.Parse(v.ChannelExpr(`getqflist({"idx":0})`), &qflist)
		if qflist.Idx == 0 {
			goto NewIndexSet
		}
		wantIdx := qflist.Idx - 1
		if len(v.lastQuickFixDiagnostics) <= wantIdx {
			goto NewIndexSet
		}
		currFix := v.lastQuickFixDiagnostics[wantIdx]
		var fileNextIdx, fileLastIdx, dirFirstIdx int
		for i, f := range fixes {
			if f.Filename == currFix.Filename {
				// Track index of the last entry of currFix file
				fileLastIdx = i + 1
				if fileNextIdx == 0 && f.Lnum >= currFix.Lnum {
					// Track index of next entry of currFix file
					fileNextIdx = i + 1
				}
			}
			if dirFirstIdx == 0 && path.Dir(f.Filename) == path.Dir(currFix.Filename) {
				// Track index of the first entry of currFix directory
				dirFirstIdx = i + 1
			}
			if currFix.equalModuloBuffer(f) {
				newIdx = i + 1
				break
			}
		}
		if newIdx == 0 {
			// If currFix isn't found, set index to the next entry from the same file
			newIdx = fileNextIdx
		}
		if newIdx == 0 {
			// If fileNextIdx isn't set, set index to the last entry from the same file
			newIdx = fileLastIdx
		}
		if newIdx == 0 {
			// If fileLastIdx isn't set, set index to the first entry from the same directory
			newIdx = dirFirstIdx
		}
	}
NewIndexSet:
	v.setQuickfixDiagnostics(fixes, newIdx)
	return nil
}

// setQuickfixDiagnostics fills quickfix list with diagnostics, and set the title and index (if != 0).
func (v *vimstate) setQuickfixDiagnostics(diags []quickfixEntry, index int) {
	v.lastQuickFixDiagnostics = diags
	v.BatchStart()
	v.BatchChannelCall("setqflist", diags, "r")
	v.BatchChannelCall("setqflist", []quickfixEntry{}, "r", qflistProps{Title: quickfixDiagnosticsTitle, Idx: index})
	v.MustBatchEnd()
}

type qflistProps struct {
	Idx   int    `json:"idx,omitempty"`
	Title string `json:"title,omitempty"`
}
