package main

import (
	"context"
	"fmt"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
)

func (v *vimstate) implements(flags govim.CommandFlags, args ...string) error {
	v.quickfixIsDiagnostics = false
	b, pos, err := v.bufCursorPos()
	if err != nil {
		return fmt.Errorf("failed to get current position: %v", err)
	}

	params := &protocol.ImplementationParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(b.URI()),
			},
			Position: pos.ToPosition(),
		},
	}

	implts, err := v.server.Implementation(context.Background(), params)
	if err != nil {
		return fmt.Errorf("call to gopls.Implementation failed: %v", err)
	}
	if len(implts) == 0 {
		return fmt.Errorf("unexpected zero length of implementations")
	}

	v.populateQuickfix(implts, false)
	return nil
}
