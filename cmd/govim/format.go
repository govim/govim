package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/config"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/types"
)

func (v *vimstate) formatCurrentBuffer(args ...json.RawMessage) (err error) {
	// we are an autocmd endpoint so we need to be told the current
	// buffer number via <abuf>
	currBufNr := v.ParseInt(args[0])
	b, ok := v.buffers[currBufNr]
	if !ok {
		return fmt.Errorf("failed to resolve buffer %v", currBufNr)
	}
	tool := v.config.FormatOnSave
	// TODO we should move this validation elsewhere...
	switch tool {
	case config.FormatOnSaveNone:
		return nil
	case config.FormatOnSaveGoFmt, config.FormatOnSaveGoImports:
	default:
		return fmt.Errorf("unknown format tool specified: %v", tool)
	}
	return v.formatBufferRange(b, tool, govim.CommandFlags{})
}

func (v *vimstate) gofmtCurrentBufferRange(flags govim.CommandFlags, args ...string) error {
	return v.formatCurrentBufferRange(config.FormatOnSaveGoFmt, flags, args...)
}

func (v *vimstate) goimportsCurrentBufferRange(flags govim.CommandFlags, args ...string) error {
	return v.formatCurrentBufferRange(config.FormatOnSaveGoImports, flags, args...)
}

func (v *vimstate) formatCurrentBufferRange(mode config.FormatOnSave, flags govim.CommandFlags, args ...string) error {
	vp := v.Viewport()
	b, ok := v.buffers[vp.Current.BufNr]
	if !ok {
		return fmt.Errorf("failed to resolve current buffer %v", vp.Current.BufNr)
	}
	return v.formatBufferRange(b, mode, flags)
}

func (v *vimstate) formatBufferRange(b *types.Buffer, mode config.FormatOnSave, flags govim.CommandFlags, args ...string) error {
	var err error
	var edits []protocol.TextEdit

	var ran *protocol.Range
	if flags.Range != nil {
		start, err := types.PointFromVim(b, *flags.Line1, 1)
		if err != nil {
			return fmt.Errorf("failed to convert start of range (%v, 1) to Point: %v", *flags.Line1, err)
		}
		end, err := types.PointFromVim(b, *flags.Line2+1, 1)
		if err != nil {
			return fmt.Errorf("failed to convert end of range (%v, 1) to Point: %v", *flags.Line2, err)
		}
		ran = &protocol.Range{
			Start: start.ToPosition(),
			End:   end.ToPosition(),
		}
	}

	switch mode {
	case config.FormatOnSaveGoFmt:
		if flags.Range != nil {
			params := &protocol.DocumentRangeFormattingParams{
				TextDocument: b.ToTextDocumentIdentifier(),
				Range:        *ran,
			}
			edits, err = v.server.RangeFormatting(context.Background(), params)
			if err != nil {
				v.Logf("gopls.RangeFormatting returned an error; nothing to do")
				return nil
			}
		} else {
			params := &protocol.DocumentFormattingParams{
				TextDocument: b.ToTextDocumentIdentifier(),
			}
			edits, err = v.server.Formatting(context.Background(), params)
			if err != nil {
				v.Logf("gopls.Formatting returned an error; nothing to do")
				return nil
			}
		}
	case config.FormatOnSaveGoImports:
		params := &protocol.CodeActionParams{
			TextDocument: b.ToTextDocumentIdentifier(),
		}
		if flags.Range != nil {
			params.Range = *ran
		}
		actions, err := v.server.CodeAction(context.Background(), params)
		if err != nil {
			v.Logf("gopls.CodeAction returned an error; nothing to do")
			return nil
		}
		switch len(actions) {
		case 0:
			return nil
		case 1:
			edits = (*actions[0].Edit.Changes)[string(b.URI())]
		default:
			return fmt.Errorf("don't know how to handle %v actions", len(actions))
		}
	default:
		return fmt.Errorf("unknown format mode specified: %v", mode)
	}
	if len(edits) == 0 {
		return nil
	}
	return v.applyProtocolTextEdits(b, edits)
}
