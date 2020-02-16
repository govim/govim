// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsp

import (
	"context"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source"
)

func (s *Server) implementation(ctx context.Context, params *protocol.ImplementationParams) ([]protocol.Location, error) {
	snapshot, fh, ok, err := s.beginFileRequest(params.TextDocument.URI, source.Go)
	if !ok {
		return nil, err
	}
	return source.Implementation(ctx, snapshot, fh, params.Position)
}
