// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsp

import (
	"context"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/telemetry/log"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/telemetry/tag"
)

func (s *Server) signatureHelp(ctx context.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	snapshot, fh, ok, err := s.beginFileRequest(params.TextDocument.URI, source.Go)
	if !ok {
		return nil, err
	}
	info, activeParameter, err := source.SignatureHelp(ctx, snapshot, fh, params.Position)
	if err != nil {
		log.Print(ctx, "no signature help", tag.Of("At", params.Position), tag.Of("Failure", err))
		return nil, nil
	}
	return &protocol.SignatureHelp{
		Signatures:      []protocol.SignatureInformation{*info},
		ActiveParameter: float64(activeParameter),
	}, nil
}
