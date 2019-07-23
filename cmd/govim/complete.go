package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/types"
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

		// Slightly bizarre (and I'm not entirely sure how it would/should work)
		// but each returned completion item can specify its own completion start
		// point. Vim does not appear to support something like this, so for now
		// let's just see how far we can get by taking the range of the first item

		start := pos.Col()
		if len(res.Items) > 0 {
			pos, err := types.PointFromPosition(b, res.Items[0].TextEdit.Range.Start)
			if err != nil {
				return nil, fmt.Errorf("failed to derive completion start: %v", err)
			}
			start = pos.Col() - 1 // see help complete-functions
		}
		v.lastCompleteResults = res
		return start, nil
	} else {
		var matches []completionResult
		for _, i := range v.lastCompleteResults.Items {
			matches = append(matches, completionResult{
				Abbr: i.Label,
				Menu: i.Detail,
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
	Menu string `json:"menu"`
}
