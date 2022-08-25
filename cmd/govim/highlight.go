package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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

	v.BatchChannelCall("prop_type_add", config.HighlightSignature, propDict{
		Highlight: string(config.HighlightSignature),
		Combine:   true,
		Priority:  types.SeverityPriority[types.SeverityErr] + 1,
	})

	v.BatchChannelCall("prop_type_add", config.HighlightSignatureParam, propDict{
		Highlight: string(config.HighlightSignatureParam),
		Combine:   true,
		Priority:  types.SeverityPriority[types.SeverityErr] + 1,
	})

	for key, hi := range types.SemanticTokenHighlight {
		v.BatchChannelCall("prop_type_add", key, propDict{
			Highlight: hi,
			Combine:   true,
		})
	}

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

// visibleLines is called each time the range of visible lines changes for anyone of the visible
// buffers (i.e. buffers with windows).
func (v *vimstate) visibleLines(args ...json.RawMessage) (interface{}, error) {
	if v.config.ExperimentalSemanticTokens == nil || !*v.config.ExperimentalSemanticTokens {
		return nil, nil
	}

	// The format used is { <winid> : { "<first>,<last>": <bufnr>, ... }, ... }
	var windows map[int]map[string]int
	v.Parse(args[0], &windows)
	for winid, rngs := range windows {
		var from, to int
		for rng, bufNr := range rngs {
			b, ok := v.buffers[bufNr]
			if !ok {
				continue
			}

			r := strings.Split(rng, ",")
			first, err := strconv.Atoi(r[0])
			if err != nil {
				return nil, fmt.Errorf("failed to parse first line nr: %v", err)
			}
			if from == 0 || first < from {
				from = first
			}
			last, err := strconv.Atoi(r[1])
			if err != nil {
				return nil, fmt.Errorf("failed to parse last line nr: %v", err)
			}
			if to == 0 || last > to {
				to = last
			}

			fp, err := types.PointFromVim(b, from, 1)
			if err != nil {
				return nil, fmt.Errorf("failed to get point from first line nr: %v", err)
			}
			tp, err := types.PointFromVim(b, to+1, 1)
			if err != nil {
				return nil, fmt.Errorf("failed to get point from last line nr: %v", err)
			}
			v.updateSemanticTokensRange(b, fp, tp, winid)
		}
	}
	return nil, nil
}

func (v *vimstate) updateSemanticTokens(b *types.Buffer) error {
	if v.config.ExperimentalSemanticTokens == nil || !*v.config.ExperimentalSemanticTokens {
		return nil
	}

	var visible struct {
		FirstLine int `json:"first"`
		LastLine  int `json:"last"`
		WinID     int `json:"winid"`
	}
	v.Parse(v.ChannelExpr(`{"first": line("w0"), "last": line("w$"), "winid": win_getid()}`), &visible)
	start, err := types.PointFromVim(b, visible.FirstLine, 1)
	if err != nil {
		return err
	}
	end, err := types.PointFromVim(b, visible.LastLine+1, 1)
	if err != nil {
		return err
	}
	v.updateSemanticTokensRange(b, start, end, visible.WinID)
	return nil
}

func (v *vimstate) updateSemanticTokensRange(b *types.Buffer, from, to types.Point, winID int) {
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel any outgoing request to make sure that we only process the
	// latest response.
	v.cancelSemTokRangeLock.Lock()
	c, ok := v.cancelSemTokRangeBuf[winID]
	if ok {
		c()
		delete(v.cancelSemTokRangeBuf, winID)
	}
	v.cancelSemTokRangeBuf[winID] = cancel
	v.cancelSemTokRangeLock.Unlock()

	v.tomb.Go(func() error {
		v.redefineSemTok(ctx, b, from, to, winID)
		return nil
	})
}

func (g *govimplugin) redefineSemTok(ctx context.Context, b *types.Buffer, start, end types.Point, winID int) error {
	select {
	case <-ctx.Done():
		return nil
	default:
	}
	semTok, err := g.server.SemanticTokensRange(ctx, &protocol.SemanticTokensRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentURI(b.URI()),
		},
		Range: protocol.Range{
			Start: start.ToPosition(),
			End:   end.ToPosition(),
		},
	})
	if err != nil {
		return err
	}
	if semTok == nil {
		return nil
	}

	g.Schedule(func(govim.Govim) error {
		// If the context is cancelled, a new SemanticTokensRange request has or will soon be sent and
		// this one is no longer relevant so we just return here.
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		v := g.vimstate
		v.BatchStart()
		defer v.BatchCancelIfNotEnded()

		// When removing previously placed tokens we must keep those visible in other windows (that
		// shows the buffer), to avoid flicker. To achieve this we define a line number range and
		// then "cut" it into multiple segments of ranges to remove.
		// E.g.:
		// Buffer 1 is visible in two different windows:
		// - Win ID 1001 shows line 100 to 110
		// - Win ID 1002 shows line 50 to 150
		// If the user changes visible range in window 1002 to line 250-350 we must clear previously
		// visible tokens on line 50 to 150, except for those placed on line 100 to 110.
		//
		// That gives us two segments to remove: 50-99 and 111-150.
		if existing, ok := v.placedSemanticTokens[winID]; ok {
			toRemove := [][2]int{{existing.from, existing.to}}
			for wi, placed := range v.placedSemanticTokens {
				if wi == winID || placed.bufnr != b.Num {
					continue
				}
				toRemove = cut(toRemove, placed.from, placed.to)
			}

			for _, remove := range toRemove {
				v.BatchChannelCall("prop_remove", struct {
					ID    int `json:"id"`
					BufNr int `json:"bufnr"`
					All   int `json:"all"`
				}{int(types.SemanticTokenPropID), b.Num, 1}, remove[0], remove[1])
			}

			delete(v.placedSemanticTokens, winID)
		}
		v.placedSemanticTokens[winID] = struct {
			bufnr int
			from  int
			to    int
		}{
			bufnr: b.Num,
			from:  start.Line(),
			to:    end.Line(),
		}

		v.handleSemanticTokens(b, semTok)
		v.MustBatchEnd()

		return nil
	})
	return nil
}

// cut turns a range, represented as a slice of from-to segments, into a new
// slice of segments where the provided range has been removed.
// e.g.
//
// If you start with a range from 1 to 10 ([1, 10]) and cut from 3 to 4 it
// returns two segments, [1,2] and [5,10].
// Cut those two from 7 to 8 returns [1,2], [5,6] and [9,10].
func cut(segments [][2]int, from int, to int) [][2]int {
	out := make([][2]int, 0)
	for _, seg := range segments {
		segFrom := seg[0]
		segTo := seg[1]

		// skip the segment if it is completely inside the cut range
		if from <= segFrom && to >= segTo {
			continue
		}

		// trim right and/or left side of this segment
		trimRight := from >= segFrom && from <= segTo
		trimLeft := to >= segFrom && to <= segTo
		if trimRight {
			out = append(out, [2]int{segFrom, from - 1})
		}
		if trimLeft {
			out = append(out, [2]int{to + 1, segTo})
		}

		// not trimmed so we keep it as is
		if !trimRight && !trimLeft {
			out = append(out, seg)
		}
	}
	return out
}

func (v *vimstate) handleSemanticTokens(b *types.Buffer, semTok *protocol.SemanticTokens) {
	// LSP 3.16 only support one format for semantic tokens, "relative". It encodes each token as
	// five uintegers according to:
	//
	// | line | start char | length | type | modifiers | ...
	//
	// Line is relative to previous token, and start char is relative to previous token (relative to
	// 0 or the previous token's start if they are on the same line), e.g.:
	//
	// | 1 | 2 | 1 | 1 | 0 |      // Line 1, Char 2,  Length 1, Type 1, Modifier 0
	// | 2 | 5 | 1 | 1 | 0 |      // Line 2, Char 5,  Length 1, Type 1, Modifier 0
	// | 0 | 7 | 1 | 1 | 0 |      // Line 2, Char 12, Length 1, Type 1, Modifier 0
	//
	if len(semTok.Data)%5 != 0 {
		v.Errorf("unexpected data length: %d", len(semTok.Data))
		return
	}

	v.semanticTokens.lock.Lock()
	defer v.semanticTokens.lock.Unlock()

	var line, char uint32
	for i := 0; i < len(semTok.Data); i += 5 {
		dline, dchar, length, tt, tm := semTok.Data[i], semTok.Data[i+1], semTok.Data[i+2], semTok.Data[i+3], semTok.Data[i+4]

		if dline != 0 {
			char = 0
		}
		line += dline
		char += dchar

		var ttype string
		var tmod []string
		if v, ok := v.semanticTokens.types[tt]; ok {
			ttype = v
		}
		for mask, mod := range v.semanticTokens.mods {
			if tm&mask > 0 {
				tmod = append(tmod, mod)
			}
		}

		pos, err := types.PointFromPosition(b, protocol.Position{
			Line:      line,
			Character: char,
		})
		if err != nil {
			v.Errorf("failed to derive start point: %v", err)
			return
		}
		end, err := types.PointFromOffset(b, pos.Offset()+int(length))
		if err != nil {
			v.Errorf("failed to derive end point: %v", err)
			return
		}
		if _, ok := types.SemanticTokenHighlight[ttype]; ok {
			v.BatchAssertChannelCall(assertPropAdd, "prop_add",
				pos.Line(),
				pos.Col(),
				propAddDict{ttype, types.SemanticTokenPropID, end.Line(), end.Col(), b.Num},
			)
		} else {
			v.Logf("missing highlight for token type %q (modifier %q) at line: %d char: %d len: %d", ttype, tmod, pos.Line(), pos.Col(), length)
		}
	}
}
