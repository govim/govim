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
