// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/ast/astutil"
	"github.com/myitcv/govim/cmd/govim/internal/span"
)

// IdentifierInfo holds information about an identifier in Go source.
type IdentifierInfo struct {
	Name  string
	Range span.Range
	File  File
	Type  struct {
		Range  span.Range
		Object types.Object
	}
	Declaration struct {
		Range  span.Range
		Node   ast.Decl
		Object types.Object
	}

	ident            *ast.Ident
	wasEmbeddedField bool
}

// Identifier returns identifier information for a position
// in a file, accounting for a potentially incomplete selector.
func Identifier(ctx context.Context, v View, f File, pos token.Pos) (*IdentifierInfo, error) {
	if result, err := identifier(ctx, v, f, pos); err != nil || result != nil {
		return result, err
	}
	// If the position is not an identifier but immediately follows
	// an identifier or selector period (as is common when
	// requesting a completion), use the path to the preceding node.
	result, err := identifier(ctx, v, f, pos-1)
	if result == nil && err == nil {
		err = fmt.Errorf("no identifier found")
	}
	return result, err
}

// identifier checks a single position for a potential identifier.
func identifier(ctx context.Context, v View, f File, pos token.Pos) (*IdentifierInfo, error) {
	fAST := f.GetAST(ctx)
	pkg := f.GetPackage(ctx)
	if pkg == nil || pkg.IsIllTyped() {
		return nil, fmt.Errorf("package for %s is ill typed", f.URI())
	}

	path, _ := astutil.PathEnclosingInterval(fAST, pos, pos)
	if path == nil {
		return nil, fmt.Errorf("can't find node enclosing position")
	}

	// Handle import specs first because they can contain *ast.Idents, and
	// we don't want the default *ast.Ident behavior below.
	if result, err := checkImportSpec(f, fAST, pkg, pos); result != nil || err != nil {
		return result, err
	}

	result := &IdentifierInfo{
		File: f,
	}

	switch node := path[0].(type) {
	case *ast.Ident:
		result.ident = node
	case *ast.SelectorExpr:
		result.ident = node.Sel
	}
	if result.ident == nil {
		return nil, nil
	}
	for _, n := range path[1:] {
		if field, ok := n.(*ast.Field); ok {
			result.wasEmbeddedField = len(field.Names) == 0
			break
		}
	}
	result.Name = result.ident.Name
	result.Range = span.NewRange(v.FileSet(), result.ident.Pos(), result.ident.End())
	result.Declaration.Object = pkg.GetTypesInfo().ObjectOf(result.ident)
	if result.Declaration.Object == nil {
		return nil, fmt.Errorf("no object for ident %v", result.Name)
	}
	if result.wasEmbeddedField {
		// The original position was on the embedded field declaration, so we
		// try to dig out the type and jump to that instead.
		if v, ok := result.Declaration.Object.(*types.Var); ok {
			if typObj := typeToObject(v.Type()); typObj != nil {
				result.Declaration.Object = typObj
			}
		}
	}
	var err error
	if result.Declaration.Range, err = objToRange(ctx, v, result.Declaration.Object); err != nil {
		return nil, err
	}
	if result.Declaration.Node, err = objToNode(ctx, v, result.Declaration.Object, result.Declaration.Range); err != nil {
		return nil, err
	}
	typ := pkg.GetTypesInfo().TypeOf(result.ident)
	if typ == nil {
		return nil, fmt.Errorf("no type for %s", result.Name)
	}
	result.Type.Object = typeToObject(typ)
	if result.Type.Object != nil {
		// Identifiers with the type "error" are a special case with no position.
		if types.IsInterface(result.Type.Object.Type()) && result.Type.Object.Pkg() == nil && result.Type.Object.Name() == "error" {
			return result, nil
		}
		if result.Type.Range, err = objToRange(ctx, v, result.Type.Object); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func checkImportSpec(f File, fAST *ast.File, pkg Package, pos token.Pos) (*IdentifierInfo, error) {
	// Check if pos is in an *ast.ImportSpec.
	for _, imp := range fAST.Imports {
		if imp.Pos() <= pos && pos < imp.End() {
			pkgPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return nil, fmt.Errorf("import path not quoted: %s (%v)", imp.Path.Value, err)
			}

			result := &IdentifierInfo{
				File:  f,
				Name:  pkgPath,
				Range: span.NewRange(f.View().FileSet(), imp.Pos(), imp.End()),
			}

			// Consider the definition of an import spec to be the imported package.
			result.Declaration.Range, err = importedPkg(f.View(), pkg, pkgPath)
			if err != nil {
				return nil, err
			}

			return result, nil
		}
	}

	return nil, nil
}

func importedPkg(v View, pkg Package, importPath string) (span.Range, error) {
	otherPkg := pkg.GetImport(importPath)
	if otherPkg == nil {
		return span.Range{}, fmt.Errorf("no import for %q", importPath)
	}
	if otherPkg.GetSyntax() == nil {
		return span.Range{}, fmt.Errorf("no syntax for for %q", importPath)
	}

	// Heuristic: Jump to the longest file of the package, assuming it's the most "interesting."
	// TODO: Consider alternative approaches, if necessary.
	var longest *ast.File
	for _, astFile := range otherPkg.GetSyntax() {
		if longest == nil || astFile.End()-astFile.Pos() > longest.End()-longest.Pos() {
			longest = astFile
		}
	}
	if longest == nil {
		return span.Range{}, fmt.Errorf("package %q has no files", importPath)
	}

	return span.NewRange(v.FileSet(), longest.Name.Pos(), longest.Name.End()), nil
}

func typeToObject(typ types.Type) types.Object {
	switch typ := typ.(type) {
	case *types.Named:
		return typ.Obj()
	case *types.Pointer:
		return typeToObject(typ.Elem())
	default:
		return nil
	}
}

func objToRange(ctx context.Context, v View, obj types.Object) (span.Range, error) {
	p := obj.Pos()
	if !p.IsValid() {
		return span.Range{}, fmt.Errorf("invalid position for %v", obj.Name())
	}
	return span.NewRange(v.FileSet(), p, p+token.Pos(len(obj.Name()))), nil
}

func objToNode(ctx context.Context, v View, obj types.Object, rng span.Range) (ast.Decl, error) {
	s, err := rng.Span()
	if err != nil {
		return nil, err
	}
	declFile, err := v.GetFile(ctx, s.URI())
	if err != nil {
		return nil, err
	}
	declAST := declFile.GetAST(ctx)
	path, _ := astutil.PathEnclosingInterval(declAST, rng.Start, rng.End)
	if path == nil {
		return nil, fmt.Errorf("no path for range %v", rng)
	}
	for _, node := range path {
		switch node := node.(type) {
		case *ast.GenDecl:
			// Type names, fields, and methods.
			switch obj.(type) {
			case *types.TypeName, *types.Var, *types.Const, *types.Func:
				return node, nil
			}
		case *ast.FuncDecl:
			// Function signatures.
			if _, ok := obj.(*types.Func); ok {
				return node, nil
			}
		}
	}
	return nil, nil // didn't find a node, but don't fail
}
