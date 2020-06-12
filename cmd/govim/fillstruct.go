package main

import (
	"context"
	"fmt"
	"math"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

func (v *vimstate) fillStruct(flags govim.CommandFlags, args ...string) error {
	b, point, err := v.bufCursorPos()
	if err != nil {
		return fmt.Errorf("failed to determine cursor position: %v", err)
	}

	textDoc := b.ToTextDocumentIdentifier()
	params := &protocol.CodeActionParams{
		TextDocument: textDoc,
		Range:        protocol.Range{Start: point.ToPosition(), End: point.ToPosition()},
		Context: protocol.CodeActionContext{
			Only: []protocol.CodeActionKind{protocol.RefactorRewrite},
		},
	}

	codeActions, err := v.server.CodeAction(context.Background(), params)
	if err != nil {
		return fmt.Errorf("codeAction failed: %v", err)
	}

	switch len(codeActions) {
	case 0:
	case 1:
		// there should be just a single file
		dcs := codeActions[0].Edit.DocumentChanges
		switch len(dcs) {
		case 1:
			dc := dcs[0]
			// verify that the URI and version of the edits match the buffer
			euri := span.URI(dc.TextDocument.TextDocumentIdentifier.URI)
			buri := b.URI()
			if euri != buri {
				return fmt.Errorf("got edits for file %v, but buffer is %v", euri, buri)
			}
			if ev := int(math.Round(dc.TextDocument.Version)); ev > 0 && ev != b.Version {
				return fmt.Errorf("got edits for version %v, but current buffer version is %v", ev, b.Version)
			}
			edits := dc.Edits
			if len(edits) != 0 {
				if err := v.applyProtocolTextEdits(b, edits); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("expected single file, saw: %v", len(dcs))
		}
	default:
		return fmt.Errorf("don't know how to handle %v actions", len(codeActions))
	}

	return nil
}
