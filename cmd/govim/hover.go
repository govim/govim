package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
)

func (v *vimstate) hover(args ...json.RawMessage) (interface{}, error) {
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
