package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/types"
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
	if tool == nil {
		return nil
	}
	return v.formatBufferRange(b, *tool, govim.CommandFlags{})
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

	switch mode {
	case config.FormatOnSaveNone:
		return nil
	case config.FormatOnSaveGoFmt, config.FormatOnSaveGoImports, config.FormatOnSaveGoImportsGoFmt:
	default:
		return fmt.Errorf("unknown format mode specified: %v", mode)
	}

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

	if mode == config.FormatOnSaveGoImports || mode == config.FormatOnSaveGoImportsGoFmt {
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
		var organizeImports []protocol.CodeAction
		// We might get other kinds in the response, like QuickFix for example.
		// They will be handled via issue #510 (add/enable support for suggested fixes)
		for _, action := range actions {
			if action.Kind == protocol.SourceOrganizeImports {
				organizeImports = append(organizeImports, action)
			}
		}

		switch len(organizeImports) {
		case 0:
		case 1:
			// there should just be a single file
			dcs := organizeImports[0].Edit.DocumentChanges
			switch len(dcs) {
			case 1:
				dc := dcs[0]
				// verify that the URI and version of the edits match the buffer
				euri := span.URI(dc.TextDocument.TextDocumentIdentifier.URI)
				buri := b.URI()
				if euri != buri {
					return fmt.Errorf("got edits for file %v, but buffer is %v", euri, buri)
				}
				if ev := dc.TextDocument.Version; ev > 0 && ev != b.Version {
					return fmt.Errorf("got edits for version %v, but current buffer version is %v", ev, b.Version)
				}
				edits := dc.Edits
				if len(edits) != 0 {
					if err := v.applyProtocolTextEdits(b, edits); err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("expected single file, saw %v", len(dcs))
			}
		default:
			return fmt.Errorf("don't know how to handle %v actions", len(organizeImports))
		}
	}
	if mode == config.FormatOnSaveGoFmt || mode == config.FormatOnSaveGoImportsGoFmt {
		var edits []protocol.TextEdit
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
		if len(edits) != 0 {
			return v.applyProtocolTextEdits(b, edits)
		}
	}
	return nil
}
