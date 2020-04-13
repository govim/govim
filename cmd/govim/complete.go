package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/types"
)

func (v *vimstate) complete(args ...json.RawMessage) (interface{}, error) {
	// Params are: findstart int, base string
	findstart := v.ParseInt(args[0]) == 1

	if findstart {
		b, pos, err := v.bufCursorPos()
		if err != nil {
			return nil, fmt.Errorf("failed to get current position: %v", err)
		}
		params := &protocol.CompletionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI(b.URI()),
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
		if v.config.ExperimentalWorkaroundCompleteoptLongest != nil && *v.config.ExperimentalWorkaroundCompleteoptLongest {
			if len(res.Items) <= 1 {
				v.ChannelEx("set completeopt-=noinsert")
				v.ChannelEx("set completeopt-=noselect")
			} else {
				v.ChannelEx("set completeopt+=noinsert")
				v.ChannelEx("set completeopt+=noselect")
			}
		}
		v.lastCompleteResults = res
		return start, nil
	} else {
		var matches []govim.CompleteItem
		for _, i := range v.lastCompleteResults.Items {
			matches = append(matches, govim.CompleteItem{
				Abbr:     i.Label,
				Menu:     i.Detail,
				Word:     i.TextEdit.NewText,
				Info:     i.Documentation,
				UserData: "govim",
			})
		}

		return matches, nil
	}
}

func (v *vimstate) completeDone(args ...json.RawMessage) error {
	currBufNr := v.ParseInt(args[0])
	b, ok := v.buffers[currBufNr]
	if !ok {
		return fmt.Errorf("failed to resolve buffer %v", currBufNr)
	}
	var chosen govim.CompleteItem
	v.Parse(args[1], &chosen)
	if chosen.Word == "" {
		return nil
	}
	if chosen.UserData != "govim" {
		return nil
	}
	var match *protocol.CompletionItem
	for _, c := range v.lastCompleteResults.Items {
		if c.Label == chosen.Abbr {
			match = &c
			break
		}
	}
	if match == nil {
		return fmt.Errorf("failed to find match for completed item %#v", chosen)
	}
	if len(match.AdditionalTextEdits) == 0 {
		return nil
	}
	return v.applyProtocolTextEdits(b, match.AdditionalTextEdits)
}
