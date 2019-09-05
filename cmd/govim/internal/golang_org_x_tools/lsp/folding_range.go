package lsp

import (
	"context"

	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/source"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/span"
)

func (s *Server) foldingRange(ctx context.Context, params *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view := s.session.ViewOf(uri)
	f, err := getGoFile(ctx, view, uri)
	if err != nil {
		return nil, err
	}
	m, err := getMapper(ctx, f)
	if err != nil {
		return nil, err
	}

	ranges, err := source.FoldingRange(ctx, view, f, s.lineFoldingOnly)
	if err != nil {
		return nil, err
	}
	return source.ToProtocolFoldingRanges(m, ranges)
}
