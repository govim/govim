package main

import (
	"path"
	"path/filepath"

	"github.com/govim/govim"
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
	wasNotDiagnostics := !v.quickfixIsDiagnostics
	v.quickfixIsDiagnostics = true
	return v.updateQuickfixWithDiagnostics(true, wasNotDiagnostics)
}

// updateQuickfixWithDiagnostics updates Vim's quickfix window with the current
// diagnostics(), respecting config settings that are overridden by force.  The
// rather clumsily named wasPrevNotDiagnostics can be set to true if it is
// known to the caller that the quickfix window was previously (to this call)
// being used for something other than diagnostics, e.g. references.
func (v *vimstate) updateQuickfixWithDiagnostics(force bool, wasPrevNotDiagnostics bool) error {
	if !force && (v.config.QuickfixAutoDiagnostics == nil || !*v.config.QuickfixAutoDiagnostics) {
		return nil
	}
	diagsRef := v.diagnostics()
	work := v.lastDiagnosticsQuickfix != diagsRef
	v.lastDiagnosticsQuickfix = diagsRef
	if (!force && !work) || !v.quickfixIsDiagnostics {
		return nil
	}
	diags := *diagsRef

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
	//
	// If we were previously not showing diagnostics, we default to selection
	// the first entry. In the future we might want to improve this logic
	// by stashing the last selected diagnostic when we flip to, for example,
	// references mode. But for now we keep it simple.
	newIdx := 0
	if !wasPrevNotDiagnostics && len(v.lastQuickFixDiagnostics) > 0 {
		var want qflistWant
		v.Parse(v.ChannelExpr(`getqflist({"idx":0})`), &want)
		if want.Idx == 0 {
			// NOTE: this should never happen since idx == 0 only if the qf is empty
			// and we just tested it isn't.
			goto NewIndexSet
		}
		wantIdx := want.Idx - 1
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
	v.lastQuickFixDiagnostics = fixes
	v.BatchStart()
	v.BatchChannelCall("setqflist", fixes, "r")
	if newIdx > 0 {
		v.BatchChannelCall("setqflist", []quickfixEntry{}, "r", qflistWant{Idx: newIdx})
	}
	v.MustBatchEnd()

	return nil
}

type qflistWant struct {
	Idx int `json:"idx"`
}
