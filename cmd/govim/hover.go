package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	if *hovRes == (protocol.Hover{}) {
		return "", nil
	}
	return strings.TrimSpace(hovRes.Contents.Value), nil
}

type popupLine struct {
	Text  string      `json:"text"`
	Props []popupProp `json:"props"`
}

type popupProp struct {
	Type   string `json:"type"`
	Col    int    `json:"col"`
	Length int    `json:"length"`
}

func (v *vimstate) showHover(posExpr string, opts map[string]interface{}, userOpts *map[string]interface{}) (interface{}, error) {
	if v.popupWinId > 0 {
		v.ChannelCall("popup_close", v.popupWinId)
		v.popupWinId = 0
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
	b, err := v.getLoadedBuffer(vpos.BufNum)
	if err != nil {
		return nil, err
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
	formatPopupline := func(msg, source string, severity types.Severity) popupLine {
		srcProp := string(config.HighlightHoverDiagSrc)
		msgProp := string(types.SeverityHoverHighlight[severity])
		return popupLine{Text: fmt.Sprintf("%s %s", msg, source),
			Props: []popupProp{
				{Type: msgProp, Col: 1, Length: len(msg) + 1 + len(source)}, // Diagnostic message
				{Type: srcProp, Col: len(msg) + 2, Length: len(source)},     // Source
			}}
	}

	var lines []popupLine
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
			lines = append(lines, popupLine{l, []popupProp{}})
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
		var err error
		if lv, ok := opts["line"]; ok {
			if line, err = rawToInt(lv); err != nil {
				return nil, fmt.Errorf("failed to parse line option: %v", err)
			}
		}
		if cv, ok := opts["col"]; ok {
			if col, err = rawToInt(cv); err != nil {
				return nil, fmt.Errorf("failed to parse col option: %v", err)
			}
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
	v.popupWinId = v.ParseInt(v.ChannelCall("popup_create", lines, opts))
	v.ChannelRedraw(false)
	return "", nil
}

func rawToInt(i interface{}) (int64, error) {
	var n json.Number
	if err := json.Unmarshal(i.(json.RawMessage), &n); err != nil {
		return 0, fmt.Errorf("failed to parse number: %v", err)
	}
	v, err := n.Int64()
	if err != nil {
		return 0, fmt.Errorf("failed to parse integer from line option: %v", err)
	}
	return v, nil
}
