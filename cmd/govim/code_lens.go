package main

import (
	"context"
	"fmt"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/command"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
)

// toggleGCDetails calls gopls CommandToggleDetails (via CodeLens) that enable/disable
// compiler annotations as diagnostics for a package. Current cursor position is used
// to determine which package to toggle.
func (v *vimstate) toggleGCDetails(flags govim.CommandFlags, args ...string) error {
	cb, _, err := v.bufCursorPos()
	if err != nil {
		return fmt.Errorf("failed to determine cursor position: %v", err)
	}
	res, err := v.server.CodeLens(context.Background(), &protocol.CodeLensParams{
		TextDocument: cb.ToTextDocumentIdentifier(),
	})
	if err != nil {
		return fmt.Errorf("codeLens failed: %v", err)
	}

	var cmd *protocol.Command
	for i := range res {
		cl := res[i]
		if cl.Command.Command != command.GCDetails.ID() {
			continue
		}
		if cmd != nil {
			return fmt.Errorf("got multiple gc_detail commands from gopls, can't handle")
		}
		cmd = &cl.Command
	}
	if cmd == nil {
		return nil
	}

	if _, err = v.server.ExecuteCommand(context.Background(),
		&protocol.ExecuteCommandParams{
			Command:   cmd.Command,
			Arguments: cmd.Arguments,
		}); err != nil {
		return fmt.Errorf("execute command failed: %v", err)
	}
	return nil
}
