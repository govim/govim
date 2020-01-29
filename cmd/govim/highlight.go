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

func (v *vimstate) redefineHighlights(diags []types.Diagnostic, force bool) error {
	if v.config.HighlightDiagnostics == nil || !*v.config.HighlightDiagnostics {
		return nil
	}
	v.diagnosticsChangedLock.Lock()
	work := v.diagnosticsChangedHighlights
	v.diagnosticsChangedHighlights = false
	v.diagnosticsChangedLock.Unlock()
	if !force && !work {
		return nil
	}

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

func (v *vimstate) updateReferenceHighlight(refresh bool) error {
	if v.config.HighlightReferences == nil || !*v.config.HighlightReferences {
		return nil
	}
	b, pos, err := v.cursorPos()
	if err != nil {
		return fmt.Errorf("failed to get current position: %v", err)
	}

	// refresh indicates if govim should call DocumentHighlight to refresh
	// ranges from gopls since we want to refresh when the user goes idle,
	// and remove highlights as soon as the user is busy. To prevent
	// flickering we keep track of the current highlight ranges and avoid
	// removing text properties if the cursor is still within all ranges.
	if !refresh {
		for i := range v.currentReferences {
			if !pos.IsWithin(*v.currentReferences[i]) {
				v.removeTextProps(types.ReferencesTextPropID)
				return nil
			}
		}
		return nil
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
		v.redefineReferenceHighlight(ctx, b, pos)
		return nil
	})

	return nil
}

func (g *govimplugin) redefineReferenceHighlight(ctx context.Context, b *types.Buffer, cursorPos types.Point) {
	res, err := g.server.DocumentHighlight(ctx,
		&protocol.DocumentHighlightParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI(b.URI()),
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

		// Same thing about user busy. If the user start moving the cursor (typing or removing text
		// for example), the results here are no longer relevant.
		if g.vimstate.userBusy {
			return nil
		}

		return g.vimstate.handleDocumentHighlight(b, cursorPos, res)
	})
}

func (v *vimstate) handleDocumentHighlight(b *types.Buffer, cursorPos types.Point, res []protocol.DocumentHighlight) error {
	if len(v.currentReferences) > 0 {
		v.currentReferences = make([]*types.Range, 0)
		v.removeTextProps(types.ReferencesTextPropID)
	}

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
			continue // We don't want to highlight what is currently under the cursor
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
