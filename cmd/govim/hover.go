package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/types"
)

func (v *vimstate) balloonExpr(args ...json.RawMessage) (interface{}, error) {
	if v.usePopupWindows() {
		posExpr := `{"bufnum": v:beval_bufnr, "line": v:beval_lnum, "col": v:beval_col, "screenpos": screenpos(v:beval_winnr, v:beval_lnum, v:beval_col)}`
		opts := map[string]interface{}{
			"mousemoved": "any",
		}
		return v.showHover(posExpr, opts, v.config.ExperimentalMouseTriggeredHoverPopupOptions)
	}
	var vpos struct {
		BufNum int `json:"bufnum"`
		Line   int `json:"line"`
		Col    int `json:"col"`
	}
	expr := v.ChannelExpr(`{"bufnum": v:beval_bufnr, "line": v:beval_lnum, "col": v:beval_col}`)
	if err := json.Unmarshal(expr, &vpos); err != nil {
		return nil, fmt.Errorf("failed to unmarshal current mouse position info: %v", err)
	}
	b, ok := v.buffers[vpos.BufNum]
	if !ok {
		return nil, fmt.Errorf("unable to resolve buffer %v", vpos.BufNum)
	}
	pos, err := types.PointFromVim(b, vpos.Line, vpos.Col)
	if err != nil {
		return nil, fmt.Errorf("failed to determine mouse position: %v", err)
	}
	go func() {
		params := &protocol.TextDocumentPositionParams{
			TextDocument: b.ToTextDocumentIdentifier(),
			Position:     pos.ToPosition(),
		}
		hovRes, err := v.server.Hover(context.Background(), params)
		if err != nil {
			v.ChannelCall("balloon_show", fmt.Sprintf("failed to get hover details: %v", err))
		} else {
			msg := strings.TrimSpace(hovRes.Contents.Value)
			var args interface{} = msg
			if !v.isGui {
				args = strings.Split(msg, "\n")
			}
			v.ChannelCall("balloon_show", args)
		}

	}()
	return "", nil
}

func (v *vimstate) hover(args ...json.RawMessage) (interface{}, error) {
	if v.usePopupWindows() {
		posExpr := `{"bufnum": bufnr(""), "line": line("."), "col": col("."), "screenpos": screenpos(winnr(), line("."), col("."))}`
		opts := map[string]interface{}{
			"mousemoved": "any",
		}
		return v.showHover(posExpr, opts, v.config.ExperimentalCursorTriggeredHoverPopupOptions)
	}
	b, pos, err := v.cursorPos()
	if err != nil {
		return nil, fmt.Errorf("failed to get current position: %v", err)
	}
	params := &protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: string(b.URI()),
		},
		Position: pos.ToPosition(),
	}
	res, err := v.server.Hover(context.Background(), params)
	if err != nil {
		return nil, fmt.Errorf("failed to get hover details: %v", err)
	}
	return strings.TrimSpace(res.Contents.Value), nil
}

func (v *vimstate) showHover(posExpr string, opts, userOpts map[string]interface{}) (interface{}, error) {
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
	b, ok := v.buffers[vpos.BufNum]
	if !ok {
		return nil, fmt.Errorf("unable to resolve buffer %v", vpos.BufNum)
	}
	pos, err := types.PointFromVim(b, vpos.Line, vpos.Col)
	if err != nil {
		return "", fmt.Errorf("failed to determine mouse position: %v", err)
	}
	params := &protocol.TextDocumentPositionParams{
		TextDocument: b.ToTextDocumentIdentifier(),
		Position:     pos.ToPosition(),
	}
	hovRes, err := v.server.Hover(context.Background(), params)
	if err != nil {
		// TODO we should only get an error when there is an error, rather than
		// nothing to display: https://github.com/golang/go/issues/32971
		v.Logf("failed to get hover details: %v", err)
		return "", nil
	}
	msg := strings.TrimSpace(hovRes.Contents.Value)
	lines := strings.Split(msg, "\n")
	if userOpts != nil {
		opts = make(map[string]interface{})
		for k, v := range userOpts {
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
		opts["pos"] = "topleft"
		opts["line"] = vpos.ScreenPos.Row + 1
		opts["col"] = vpos.ScreenPos.Col
		opts["mousemoved"] = "any"
		opts["moved"] = "any"
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
