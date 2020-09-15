// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsp

import (
	"context"
	"fmt"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/event"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/mod"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source"
)

func (s *Server) codeLens(ctx context.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	snapshot, fh, ok, release, err := s.beginFileRequest(ctx, params.TextDocument.URI, source.UnknownKind)
	defer release()
	if !ok {
		return nil, err
	}
	var lensFuncs map[string]source.LensFunc
	switch fh.Kind() {
	case source.Mod:
		lensFuncs = mod.LensFuncs()
	case source.Go:
		lensFuncs = source.LensFuncs()
	default:
		// Unsupported file kind for a code lens.
		return nil, nil
	}
	var result []protocol.CodeLens
	for lens, lf := range lensFuncs {
		if !snapshot.View().Options().Codelens[lens] {
			continue
		}
		added, err := lf(ctx, snapshot, fh)
		// Code lens is called on every keystroke, so we should just operate in
		// a best-effort mode, ignoring errors.
		if err != nil {
			event.Error(ctx, fmt.Sprintf("code lens %s failed", lens), err)
			continue
		}
		result = append(result, added...)
	}
	return result, nil
}
