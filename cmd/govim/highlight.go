package main

import (
	"context"
	"fmt"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/types"
)

// propDict is the representation of arguments used in vim's prop_type_add()
type propDict struct {
	Highlight string `json:"highlight"`
	Combine   bool   `json:"combine,omitempty"`
	Priority  int    `json:"priority,omitempty"`
	StartIncl bool   `json:"start_incl,omitempty"`
	EndIncl   bool   `json:"end_incl,omitempty"`
}

// propAddDict is the representation of arguments used in vim's prop_add()
type propAddDict struct {
	Type    string `json:"type"`
	ID      int    `json:"id"`
	EndLine int    `json:"end_lnum"`
	EndCol  int    `json:"end_col"` // Column just after the text
	BufNr   int    `json:"bufnr"`
}

// assertPropAdd is used when we add text properties that might fail due to the fact
// that the buffer might have changed since the text properties was calculated.
// There are two vim errors that we like to suppress, invalid line and invalid column.
var assertPropAdd AssertExpr = AssertIsErrorOrNil(
	"^Vim(let):E964:", // Invalid column (col) passed to vim prop_add()
	"^Vim(let):E966:") // Invalid line (lnum) passed to vim prop_add()

func (v *vimstate) textpropDefine() error {
	v.BatchStart()
	// Note that we reuse the highlight name as text property name, even if they aren't the same thing.
	for _, s := range []types.Severity{types.SeverityErr, types.SeverityWarn, types.SeverityInfo, types.SeverityHint} {
		hi := types.SeverityHighlight[s]

		v.BatchChannelCall("prop_type_add", hi, propDict{
			Highlight: string(hi),
			Combine:   true, // Combine with syntax highlight
			Priority:  types.SeverityPriority[s],
		})

		hi = types.SeverityHoverHighlight[s]
		v.BatchChannelCall("prop_type_add", hi, propDict{
			Highlight: string(hi),
			Combine:   true, // Combine with syntax highlight
			Priority:  types.SeverityPriority[s],
		})
	}

	v.BatchChannelCall("prop_type_add", config.HighlightHoverDiagSrc, propDict{
		Highlight: string(config.HighlightHoverDiagSrc),
		Combine:   true, // Combine with syntax highlight
		Priority:  types.SeverityPriority[types.SeverityErr] + 1,
	})

	v.BatchChannelCall("prop_type_add", config.HighlightReferences, propDict{
		Highlight: string(config.HighlightReferences),
		Combine:   true,
		Priority:  types.SeverityPriority[types.SeverityErr] + 1,
	})

	res := v.MustBatchEnd()
	for i := range res {
		if v.ParseInt(res[i]) != 0 {
			return fmt.Errorf("call to prop_type_add() failed")
		}
	}
	return nil
}

func (v *vimstate) redefineHighlights(force bool) error {
	if v.config.HighlightDiagnostics == nil || !*v.config.HighlightDiagnostics {
		return nil
	}
	diagsRef := v.diagnostics()
	work := v.lastDiagnosticsHighlights != diagsRef
	v.lastDiagnosticsHighlights = diagsRef
	if !force && !work {
		return nil
	}
	diags := *diagsRef

	v.removeTextProps(types.DiagnosticTextPropID)

	v.BatchStart()
	defer v.BatchCancelIfNotEnded()
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

		v.BatchAssertChannelCall(assertPropAdd, "prop_add",
			d.Range.Start.Line(),
			d.Range.Start.Col(),
			propAddDict{string(hi), types.DiagnosticTextPropID, d.Range.End.Line(), d.Range.End.Col(), d.Buf},
		)
	}

	v.MustBatchEnd()
	return nil
}

func (v *vimstate) highlightReferences(flags govim.CommandFlags, args ...string) error {
	v.highlightingReferences = true
	return v.updateReferenceHighlight(true)
}

func (v *vimstate) clearReferencesHighlights(flags govim.CommandFlags, args ...string) error {
	v.highlightingReferences = false
	return v.removeReferenceHighlight(nil)
}

// removeReferenceHighlight is used to only remove existing reference
// highlights if the cursor has moved outside of the range(s) of existing
// highlights, to avoid flickering. Passing a nil *types.CursorPosition or a
// cursorPos without a Point (i.e. not in a buffer tracked by govim) removes
// highlights regardless.
func (v *vimstate) removeReferenceHighlight(cursorPos *types.CursorPosition) error {
	if v.highlightingReferences || v.currentReferences == nil {
		return nil
	}
	if cursorPos != nil && cursorPos.Point != nil {
		for i := range v.currentReferences {
			if cursorPos.IsWithin(*v.currentReferences[i]) {
				return nil
			}
		}
	}
	v.removeTextProps(types.ReferencesTextPropID)
	v.currentReferences = nil
	return nil
}

func (v *vimstate) updateReferenceHighlight(force bool) error {
	pos, err := v.cursorPos()
	if err != nil {
		return fmt.Errorf("failed to get cursor position: %w", err)
	}
	return v.updateReferenceHighlightAtCursorPosition(force, pos)
}

// updateReferenceHighlight updates the reference highlighting if the cursor is
// currently in a valid position, i.e. a go file. The cursor position can passed
// in by supplying a non-nil cursorPos.
func (v *vimstate) updateReferenceHighlightAtCursorPosition(force bool, cursorPos types.CursorPosition) error {
	if !force && (v.config.HighlightReferences == nil || !*v.config.HighlightReferences) {
		return nil
	}

	if cursorPos.Point == nil {
		// Cursor is in a non-go buffer. Remove referenceHighlights
		return v.removeReferenceHighlight(nil)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel any ongoing requests to make sure that we only process the
	// latest response.
	v.cancelDocHighlightLock.Lock()
	if v.cancelDocHighlight != nil {
		v.cancelDocHighlight()
		v.cancelDocHighlight = nil
	}
	v.cancelDocHighlight = cancel
	v.cancelDocHighlightLock.Unlock()

	v.tomb.Go(func() error {
		v.redefineReferenceHighlight(ctx, cursorPos)
		return nil
	})

	return nil
}

func (g *govimplugin) redefineReferenceHighlight(ctx context.Context, cursorPos types.CursorPosition) {
	res, err := g.server.DocumentHighlight(ctx,
		&protocol.DocumentHighlightParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI(cursorPos.Buffer().URI()),
				},
				Position: cursorPos.ToPosition(),
			},
		},
	)

	select {
	case <-ctx.Done():
		return
	default:
	}
	if err != nil {
		g.Logf("documentHighlight call failed: %v", err)
		return
	}

	g.Schedule(func(govim.Govim) error {
		// If the context is cancelled, a new DocumentHighlight request has or will soon be sent and
		// this one is no longer relevant so we just return here.
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		return g.vimstate.handleDocumentHighlight(cursorPos, res)
	})
}

func (v *vimstate) handleDocumentHighlight(cursorPos types.CursorPosition, res []protocol.DocumentHighlight) error {
	if len(v.currentReferences) > 0 {
		v.currentReferences = nil
		v.removeTextProps(types.ReferencesTextPropID)
	}

	b := cursorPos.Buffer()

	v.BatchStart()
	defer v.BatchCancelIfNotEnded()
	for i := range res {
		start, err := types.PointFromPosition(b, res[i].Range.Start)
		if err != nil {
			v.Logf("failed to convert start position %v to point: %v", res[i].Range.Start, err)
			return nil
		}
		end, err := types.PointFromPosition(b, res[i].Range.End)
		if err != nil {
			v.Logf("failed to convert end position %v to point: %v", res[i].Range.End, err)
			return nil
		}
		r := types.Range{Start: start, End: end}
		if cursorPos.IsWithin(r) {
			v.currentReferences = append(v.currentReferences, &r)
			// If we are highighting references as a result of a direct call to
			// CommandHighlightReferences then we do want to highlight the identifier
			// under the cusor, otherwise not.
			if !v.highlightingReferences {
				continue
			}
		}
		v.BatchAssertChannelCall(assertPropAdd, "prop_add",
			start.Line(),
			start.Col(),
			propAddDict{string(config.HighlightReferences), types.ReferencesTextPropID, end.Line(), end.Col(), b.Num},
		)
	}
	v.MustBatchEnd()
	return nil
}

// removeTextProps is used to remove all added text properties with a specific ID, regardless
// of configuration setting.
func (v *vimstate) removeTextProps(id types.TextPropID) {
	var didStart bool
	if didStart = v.BatchStartIfNeeded(); didStart {
		defer v.BatchCancelIfNotEnded()
	}

	for bufnr, buf := range v.buffers {
		if !buf.Loaded {
			continue // vim removes properties when a buffer is unloaded
		}
		v.BatchChannelCall("prop_remove", struct {
			ID    int `json:"id"`
			BufNr int `json:"bufnr"`
			All   int `json:"all"`
		}{int(id), bufnr, 1})
	}

	if didStart {
		// prop_remove returns number of removed properties per call
		v.MustBatchEnd()
	}
}
