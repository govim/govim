package main

import (
	"fmt"

	"github.com/govim/govim/cmd/govim/internal/types"
)

type propDict struct {
	Highlight string `json:"highlight"`
	Combine   bool   `json:"combine,omitempty"`
	Priority  int    `json:"priority,omitempty"`
	StartIncl bool   `json:"start_incl,omitempty"`
	EndIncl   bool   `json:"end_incl,omitempty"`
}

func (v *vimstate) textpropDefine() error {
	v.BatchStart()
	for _, s := range []types.Severity{types.SeverityErr, types.SeverityWarn, types.SeverityInfo, types.SeverityHint} {
		hi := types.SeverityHighlight[s]

		// Note that we reuse the highlight name as text property name, even if they aren't the same thing.
		v.BatchChannelCall("prop_type_add", hi, propDict{
			Highlight: string(hi),
			Combine:   true, // Combine with syntax highlight
			Priority:  types.SeverityPriority[s],
		})
	}
	res := v.BatchEnd()
	for i := range res {
		if v.ParseInt(res[i]) != 0 {
			return fmt.Errorf("call to prop_type_add() failed")
		}
	}
	return nil
}

func (v *vimstate) redefineHighlights(diags []types.Diagnostic) error {
	if v.config.HighlightDiagnostics == nil || !*v.config.HighlightDiagnostics {
		return nil
	}

	v.BatchStart()
	defer v.BatchCancelIfNotEnded()
	for bufnr, buf := range v.buffers {
		if !buf.Loaded {
			continue // vim removes properties when a buffer is unloaded
		}
		v.BatchChannelCall("prop_remove", struct {
			ID    int `json:"id"`
			BufNr int `json:"bufnr"`
			All   int `json:"all"`
		}{0, bufnr, 1})
	}

	for _, d := range diags {
		// Do not add textprops to unknown buffers
		if d.Buf < 0 {
			continue
		}

		// prop_add() can only be called for Loaded buffers, otherwise
		// it will throw an "unknown line" error.
		if buf, ok := v.buffers[d.Buf]; ok && !buf.Loaded {
			continue
		}

		hi, ok := types.SeverityHighlight[d.Severity]
		if !ok {
			return fmt.Errorf("failed to find highlight for severity %v", d.Severity)
		}

		v.BatchChannelCall("prop_add",
			d.Range.Start.Line(),
			d.Range.Start.Col(),
			struct {
				Type    string `json:"type"`
				EndLine int    `json:"end_lnum"`
				EndCol  int    `json:"end_col"` // column just after the text
				BufNr   int    `json:"bufnr"`
			}{string(hi), d.Range.End.Line(), d.Range.End.Col(), d.Buf})
	}

	v.BatchEnd()
	return nil
}
