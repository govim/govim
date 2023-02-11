// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"context"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/imports"
)

// AddImport adds a single import statement to the given file
func AddImport(ctx context.Context, snapshot Snapshot, fh FileHandle, importPath string) ([]protocol.TextEdit, error) {
	pgf, err := snapshot.ParseGo(ctx, fh, ParseFull)
	if err != nil {
		return nil, err
	}
	return ComputeOneImportFixEdits(snapshot, pgf, &imports.ImportFix{
		StmtInfo: imports.ImportInfo{
			ImportPath: importPath,
		},
		FixType: imports.AddImport,
	})
}
