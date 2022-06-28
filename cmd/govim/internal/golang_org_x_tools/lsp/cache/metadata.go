// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"go/types"

	"golang.org/x/tools/go/packages"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/packagesinternal"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

// Declare explicit types for package paths, names, and IDs to ensure that we
// never use an ID where a path belongs, and vice versa. If we confused these,
// it would result in confusing errors because package IDs often look like
// package paths.
type (
	PackageID   string
	PackagePath string
	PackageName string
)

// Metadata holds package Metadata extracted from a call to packages.Load.
type Metadata struct {
	ID              PackageID
	PkgPath         PackagePath
	Name            PackageName
	GoFiles         []span.URI
	CompiledGoFiles []span.URI
	ForTest         PackagePath
	TypesSizes      types.Sizes
	Errors          []packages.Error
	Deps            []PackageID
	MissingDeps     map[PackagePath]struct{}
	Module          *packages.Module
	depsErrors      []*packagesinternal.PackageError

	// Config is the *packages.Config associated with the loaded package.
	Config *packages.Config

	// IsIntermediateTestVariant reports whether the given package is an
	// intermediate test variant, e.g.
	// "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/cache [github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source.test]".
	//
	// Such test variants arise when an x_test package (in this case source_test)
	// imports a package (in this case cache) that itself imports the the
	// non-x_test package (in this case source).
	//
	// This is done so that the forward transitive closure of source_test has
	// only one package for the "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source" import.
	// The intermediate test variant exists to hold the test variant import:
	//
	// github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source_test [github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source.test]
	//  | "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/cache" -> github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/cache [github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source.test]
	//  | "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source" -> github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source [github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source.test]
	//  | ...
	//
	// github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/cache [github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source.test]
	//  | "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source" -> github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source [github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source.test]
	//  | ...
	//
	// We filter these variants out in certain places. For example, there is
	// generally no reason to run diagnostics or analysis on them.
	//
	// TODO(rfindley): this can probably just be a method, since it is derived
	// from other fields.
	IsIntermediateTestVariant bool
}

// Name implements the source.Metadata interface.
func (m *Metadata) PackageName() string {
	return string(m.Name)
}

// PkgPath implements the source.Metadata interface.
func (m *Metadata) PackagePath() string {
	return string(m.PkgPath)
}

// ModuleInfo implements the source.Metadata interface.
func (m *Metadata) ModuleInfo() *packages.Module {
	return m.Module
}

// KnownMetadata is a wrapper around metadata that tracks its validity.
type KnownMetadata struct {
	*Metadata

	// Valid is true if the given metadata is Valid.
	// Invalid metadata can still be used if a metadata reload fails.
	Valid bool

	// PkgFilesChanged reports whether the file set of this metadata has
	// potentially changed.
	PkgFilesChanged bool

	// ShouldLoad is true if the given metadata should be reloaded.
	//
	// Note that ShouldLoad is different from !Valid: when we try to load a
	// package, we mark ShouldLoad = false regardless of whether the load
	// succeeded, to prevent endless loads.
	ShouldLoad bool
}
