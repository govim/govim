package main

import (
	"context"
	"fmt"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
)

func (v *vimstate) references(flags govim.CommandFlags, args ...string) error {
	v.quickfixIsDiagnostics = false
	b, pos, err := v.bufCursorPos()
	if err != nil {
		return fmt.Errorf("failed to get current position: %v", err)
	}
	params := &protocol.ReferenceParams{
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(b.URI()),
			},
			Position: pos.ToPosition(),
		},
	}

	refs, err := v.server.References(context.Background(), params)
	if err != nil {
		return fmt.Errorf("called to gopls.References failed: %v", err)
	}
	if len(refs) == 0 {
		return fmt.Errorf("unexpected zero length of references")
	}

	v.populateQuickfix(refs, true)
	return nil
}
