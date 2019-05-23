// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"go/token"

	"github.com/myitcv/govim/cmd/govim/internal/lsp/source"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/xlog"
	"github.com/myitcv/govim/cmd/govim/internal/span"
)

func New() source.Cache {
	return &cache{
		fset: token.NewFileSet(),
	}
}

type cache struct {
	fset *token.FileSet
}

func (c *cache) NewSession(log xlog.Logger) source.Session {
	return &session{
		cache:    c,
		log:      log,
		overlays: make(map[span.URI][]byte),
	}
}

func (c *cache) FileSet() *token.FileSet {
	return c.fset
}
