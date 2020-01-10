// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsp

import (
	"context"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

func (s *Server) definition(ctx context.Context, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view, err := s.session.ViewOf(uri)
	if err != nil {
		return nil, err
	}
	snapshot := view.Snapshot()
	fh, err := snapshot.GetFile(uri)
	if err != nil {
		return nil, err
	}
	if fh.Identity().Kind != source.Go {
		return nil, nil
	}
	ident, err := source.Identifier(ctx, snapshot, fh, params.Position, source.WidestPackageHandle)
	if err != nil {
		return nil, err
	}
	decRange, err := ident.Declaration.Range()
	if err != nil {
		return nil, err
	}
	return []protocol.Location{
		{
			URI:   protocol.NewURI(ident.Declaration.URI()),
			Range: decRange,
		},
	}, nil
}

func (s *Server) typeDefinition(ctx context.Context, params *protocol.TypeDefinitionParams) ([]protocol.Location, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view, err := s.session.ViewOf(uri)
	if err != nil {
		return nil, err
	}
	snapshot := view.Snapshot()
	fh, err := snapshot.GetFile(uri)
	if err != nil {
		return nil, err
	}
	if fh.Identity().Kind != source.Go {
		return nil, nil
	}
	ident, err := source.Identifier(ctx, snapshot, fh, params.Position, source.WidestPackageHandle)
	if err != nil {
		return nil, err
	}
	identRange, err := ident.Type.Range()
	if err != nil {
		return nil, err
	}
	return []protocol.Location{
		{
			URI:   protocol.NewURI(ident.Type.URI()),
			Range: identRange,
		},
	}, nil
}
