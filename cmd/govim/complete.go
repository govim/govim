package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
)

func (v *vimstate) complete(args ...json.RawMessage) (interface{}, error) {
	// Params are: findstart int, base string
	findstart := v.ParseInt(args[0]) == 1

	if findstart {
		b, pos, err := v.cursorPos()
		if err != nil {
			return nil, fmt.Errorf("failed to get current position: %v", err)
		}
		params := &protocol.CompletionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: string(b.URI()),
				},
				Position: pos.ToPosition(),
			},
		}
		res, err := v.server.Completion(context.Background(), params)
		if err != nil {
			return nil, fmt.Errorf("called to gopls.Completion failed: %v", err)
		}

		v.lastCompleteResults = res
		return pos.Col(), nil
	} else {
		var matches []completionResult
		for _, i := range v.lastCompleteResults.Items {
			matches = append(matches, completionResult{
				Abbr: i.Label,
				Word: i.TextEdit.NewText,
				Info: i.Detail,
			})
		}

		return matches, nil
	}
}

type completionResult struct {
	Abbr string `json:"abbr"`
	Word string `json:"word"`
	Info string `json:"info"`
}
