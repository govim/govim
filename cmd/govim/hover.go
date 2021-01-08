package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/types"
)

func (v *vimstate) balloonExpr(args ...json.RawMessage) (interface{}, error) {
	posExpr := `{"bufnum": v:beval_bufnr, "line": v:beval_lnum, "col": v:beval_col, "screenpos": screenpos(v:beval_winid, v:beval_lnum, v:beval_col)}`
	opts := map[string]interface{}{
		"mousemoved": "any",
	}
	return v.showHover(posExpr, opts, v.config.ExperimentalMouseTriggeredHoverPopupOptions)
}

func (v *vimstate) hover(args ...json.RawMessage) (interface{}, error) {
	posExpr := `{"bufnum": bufnr(""), "line": line("."), "col": col("."), "screenpos": screenpos(win_getid(), line("."), col("."))}`
	opts := map[string]interface{}{
		"mousemoved": "any",
	}
	return v.showHover(posExpr, opts, v.config.ExperimentalCursorTriggeredHoverPopupOptions)
}

func (v *vimstate) hoverMsgAt(pos types.Point, tdi protocol.TextDocumentIdentifier) (string, error) {
	params := &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: tdi,
			Position:     pos.ToPosition(),
		},
	}
	hovRes, err := v.server.Hover(context.Background(), params)
	if err != nil {
		return "", fmt.Errorf("failed to get hover details: %v", err)
	}
	if hovRes == nil || *hovRes == (protocol.Hover{}) {
		return "", nil
	}
	return strings.TrimSpace(hovRes.Contents.Value), nil
}

func (v *vimstate) showHover(posExpr string, opts map[string]interface{}, userOpts *map[string]interface{}) (interface{}, error) {
	if v.popupWinID > 0 {
		v.ChannelCall("popup_close", v.popupWinID)
		v.popupWinID = 0
		v.ChannelRedraw(false)
	}
	var vpos struct {
		BufNum    int `json:"bufnum"`
		Line      int `json:"line"`
		Col       int `json:"col"`
		ScreenPos struct {
			Row int `json:"row"`
			Col int `json:"col"`
		} `json:"screenpos"`
	}
	expr := v.ChannelExpr(posExpr)
	v.Parse(expr, &vpos)
	b, ok := v.buffers[vpos.BufNum]
	if !ok {
		return nil, fmt.Errorf("unable to resolve buffer %v", vpos.BufNum)
	}
	pos, err := types.PointFromVim(b, vpos.Line, vpos.Col)
	if err != nil {
		return "", fmt.Errorf("failed to determine mouse position: %v", err)
	}

	// formatPopupLine applies text properties to a single diagnostic based on
	// it's severity. The severity unique property is applied to the entire line,
	// while the common "source highlight" is applied to the source part. Since
	// the source highlight is combined to existing highlight (and have a higher
	// priority than the unique), it enables a wide range of styling combinations.
	formatPopupline := func(msg, source string, severity types.Severity) types.PopupLine {
		srcProp := string(config.HighlightHoverDiagSrc)
		msgProp := string(types.SeverityHoverHighlight[severity])
		return types.PopupLine{
			Text: fmt.Sprintf("%s %s", msg, source),
			Props: []types.PopupProp{
				{Type: msgProp, Col: 1, Len: len(msg) + 1 + len(source)}, // Diagnostic message
				{Type: srcProp, Col: len(msg) + 2, Len: len(source)},     // Source
			},
		}
	}

	var lines []types.PopupLine
	if *v.config.HoverDiagnostics {
		for _, d := range *v.diagnostics() {
			if (b.Num != d.Buf) || !pos.IsWithin(d.Range) {
				continue
			}
			lines = append(lines, formatPopupline(d.Text, d.Source, d.Severity))
		}
	}
	msg, err := v.hoverMsgAt(pos, b.ToTextDocumentIdentifier())
	if err != nil {
		return "", err
	}
	if msg != "" {
		for _, l := range strings.Split(msg, "\n") {
			lines = append(lines, types.PopupLine{Text: l, Props: []types.PopupProp{}})
		}
	}
	if len(lines) == 0 {
		return "", nil
	}

	if userOpts != nil {
		opts = make(map[string]interface{})
		for k, v := range *userOpts {
			opts[k] = v
		}
		var line, col int64
		// TODO: we should use json.Decoder.UseNumber() instead of treating ints as floats.
		if lv, ok := opts["line"].(float64); ok {
			line = int64(math.Round(lv))
		}
		if cv, ok := opts["col"].(float64); ok {
			col = int64(math.Round(cv))
		}
		opts["line"] = line + int64(vpos.ScreenPos.Row)
		opts["col"] = col + int64(vpos.ScreenPos.Col)
	} else {
		opts["pos"] = "botleft"
		opts["line"] = vpos.ScreenPos.Row - 1
		opts["col"] = vpos.ScreenPos.Col
		opts["mousemoved"] = "any"
		opts["moved"] = "any"
		opts["padding"] = []int{0, 1, 0, 1}
		opts["wrap"] = false
		opts["close"] = "click"
	}
	v.popupWinID = v.ParseInt(v.ChannelCall("popup_create", lines, opts))
	v.ChannelRedraw(false)
	return "", nil
}
