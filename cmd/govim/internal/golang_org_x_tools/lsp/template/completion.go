// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"context"
	"fmt"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source/completion"
)

func Completion(ctx context.Context, snapshot source.Snapshot, fh source.VersionedFileHandle, pos protocol.Position, context protocol.CompletionContext) ([]completion.CompletionItem, *completion.Selection, error) {
	if skipTemplates(snapshot) {
		return nil, nil, nil
	}
	return nil, nil, fmt.Errorf("implement template completion")
}
