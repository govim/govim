// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"context"
	"errors"
	"fmt"
	"go/token"
	"strings"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/safetoken"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/span"
)

// ErrNoLinkname is returned by LinknameDefinition when no linkname
// directive is found at a particular position.
// As such it indicates that other definitions could be worth checking.
var ErrNoLinkname = errors.New("no linkname directive found")

// LinknameDefinition finds the definition of the linkname directive in fh at pos.
// If there is no linkname directive at pos, returns ErrNoLinkname.
func LinknameDefinition(ctx context.Context, snapshot Snapshot, fh FileHandle, pos protocol.Position) ([]protocol.Location, error) {
	pkgPath, name := parseLinkname(ctx, snapshot, fh, pos)
	if pkgPath == "" {
		return nil, ErrNoLinkname
	}
	return findLinkname(ctx, snapshot, fh, pos, PackagePath(pkgPath), name)
}

// parseLinkname attempts to parse a go:linkname declaration at the given pos.
// If successful, it returns the package path and object name referenced by the second
// argument of the linkname directive.
//
// If the position is not in the second argument of a go:linkname directive, or parsing fails, it returns "", "".
func parseLinkname(ctx context.Context, snapshot Snapshot, fh FileHandle, pos protocol.Position) (pkgPath, name string) {
	pgf, err := snapshot.ParseGo(ctx, fh, ParseFull)
	if err != nil {
		return "", ""
	}

	span, err := pgf.Mapper.PositionPoint(pos)
	if err != nil {
		return "", ""
	}
	atLine := span.Line()
	atColumn := span.Column()

	// Looking for pkgpath in '//go:linkname f pkgpath.g'.
	// (We ignore 1-arg linkname directives.)
	directive, column := findLinknameOnLine(pgf, atLine)
	parts := strings.Fields(directive)
	if len(parts) != 3 {
		return "", ""
	}

	// Inside 2nd arg [start, end]?
	end := column + len(directive)
	start := end - len(parts[2])
	if !(start <= atColumn && atColumn <= end) {
		return "", ""
	}
	linkname := parts[2]

	// Split the pkg path from the name.
	dot := strings.LastIndexByte(linkname, '.')
	if dot < 0 {
		return "", ""
	}
	return linkname[:dot], linkname[dot+1:]
}

// findLinknameOnLine returns the first linkname directive on line and the column it starts at.
// Returns "", 0 if no linkname directive is found on the line.
func findLinknameOnLine(pgf *ParsedGoFile, line int) (string, int) {
	for _, grp := range pgf.File.Comments {
		for _, com := range grp.List {
			if strings.HasPrefix(com.Text, "//go:linkname") {
				p := safetoken.Position(pgf.Tok, com.Pos())
				if p.Line == line {
					return com.Text, p.Column
				}
			}
		}
	}
	return "", 0
}

// findLinkname searches dependencies of packages containing fh for an object
// with linker name matching the given package path and name.
func findLinkname(ctx context.Context, snapshot Snapshot, fh FileHandle, pos protocol.Position, pkgPath PackagePath, name string) ([]protocol.Location, error) {
	metas, err := snapshot.MetadataForFile(ctx, fh.URI())
	if err != nil {
		return nil, err
	}
	if len(metas) == 0 {
		return nil, fmt.Errorf("no package found for file %q", fh.URI())
	}

	// Find package starting from narrowest package metadata.
	pkgMeta := findPackageInDeps(snapshot, metas[0], pkgPath)
	if pkgMeta == nil {
		// Fall back to searching reverse dependencies.
		reverse, err := snapshot.ReverseDependencies(ctx, metas[0].ID, true /* transitive */)
		if err != nil {
			return nil, err
		}
		for _, dep := range reverse {
			if dep.PkgPath == pkgPath {
				pkgMeta = dep
				break
			}
		}
		if pkgMeta == nil {
			return nil, fmt.Errorf("cannot find package %q", pkgPath)
		}
	}

	// When found, type check the desired package (snapshot.TypeCheck in TypecheckFull mode),
	pkgs, err := snapshot.TypeCheck(ctx, TypecheckFull, pkgMeta.ID)
	if err != nil {
		return nil, err
	}
	pkg := pkgs[0]

	obj := pkg.GetTypes().Scope().Lookup(name)
	if obj == nil {
		return nil, fmt.Errorf("package %q does not define %s", pkgPath, name)
	}

	objURI := safetoken.StartPosition(pkg.FileSet(), obj.Pos())
	pgf, err := pkg.File(span.URIFromPath(objURI.Filename))
	if err != nil {
		return nil, err
	}
	loc, err := pgf.PosLocation(obj.Pos(), obj.Pos()+token.Pos(len(name)))
	if err != nil {
		return nil, err
	}
	return []protocol.Location{loc}, nil
}

// findPackageInDeps returns the dependency of meta of the specified package path, if any.
func findPackageInDeps(snapshot Snapshot, meta *Metadata, pkgPath PackagePath) *Metadata {
	seen := make(map[*Metadata]bool)
	var visit func(*Metadata) *Metadata
	visit = func(meta *Metadata) *Metadata {
		if !seen[meta] {
			seen[meta] = true
			if meta.PkgPath == pkgPath {
				return meta
			}
			for _, id := range meta.DepsByPkgPath {
				if m := visit(snapshot.Metadata(id)); m != nil {
					return m
				}
			}
		}
		return nil
	}
	return visit(meta)
}
