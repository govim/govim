package main

import (
	"context"
	"encoding/json"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
)

func (v *vimstate) workspaceSymbol(args ...json.RawMessage) (interface{}, error) {
	var params protocol.WorkspaceSymbolParams
	v.Parse(args[0], &params)
	return v.server.Symbol(context.Background(), &params)
}
