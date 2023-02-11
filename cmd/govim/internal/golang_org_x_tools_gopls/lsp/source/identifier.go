// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/ast/astutil"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/safetoken"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/span"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/event"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/typeparams"
)

// IdentifierInfo holds information about an identifier in Go source.
type IdentifierInfo struct {
	Name        string
	Snapshot    Snapshot // only needed for .View(); TODO(adonovan): reduce.
	MappedRange protocol.MappedRange

	Type struct {
		MappedRange protocol.MappedRange
		Object      *types.TypeName
	}

	Inferred *types.Signature

	Declaration Declaration

	ident *ast.Ident

	// For struct fields or embedded interfaces, enclosing is the object
	// corresponding to the outer type declaration, if it is exported, for use in
	// documentation links.
	enclosing *types.TypeName

	pkg Package
	qf  types.Qualifier
}

type Declaration struct {
	MappedRange []protocol.MappedRange

	// The typechecked node
	node     ast.Node
	nodeFile *ParsedGoFile // provides token.File and Mapper for node

	// Optional: the fully parsed node, to be used for formatting in cases where
	// node has missing information. This could be the case when node was parsed
	// in ParseExported mode.
	fullDecl ast.Decl

	// The typechecked object.
	obj types.Object

	// typeSwitchImplicit indicates that the declaration is in an implicit
	// type switch. Its type is the type of the variable on the right-hand
	// side of the type switch.
	typeSwitchImplicit types.Type
}

// Identifier returns identifier information for a position
// in a file, accounting for a potentially incomplete selector.
func Identifier(ctx context.Context, snapshot Snapshot, fh FileHandle, position protocol.Position) (*IdentifierInfo, error) {
	ctx, done := event.Start(ctx, "source.Identifier")
	defer done()

	pkg, pgf, err := PackageForFile(ctx, snapshot, fh.URI(), TypecheckFull, NarrowestPackage)
	if err != nil {
		return nil, err
	}
	pos, err := pgf.PositionPos(position)
	if err != nil {
		return nil, err
	}
	return findIdentifier(ctx, snapshot, pkg, pgf, pos)
}

// ErrNoIdentFound is error returned when no identifier is found at a particular position
var ErrNoIdentFound = errors.New("no identifier found")

func findIdentifier(ctx context.Context, snapshot Snapshot, pkg Package, pgf *ParsedGoFile, pos token.Pos) (*IdentifierInfo, error) {
	file := pgf.File
	// Handle import specs separately, as there is no formal position for a
	// package declaration.
	if result, err := importSpec(ctx, snapshot, pkg, pgf, pos); result != nil || err != nil {
		return result, err
	}
	path := pathEnclosingObjNode(file, pos)
	if path == nil {
		return nil, ErrNoIdentFound
	}

	qf := Qualifier(file, pkg.GetTypes(), pkg.GetTypesInfo())

	ident, _ := path[0].(*ast.Ident)
	if ident == nil {
		return nil, ErrNoIdentFound
	}
	// Special case for package declarations, since they have no
	// corresponding types.Object.
	if ident == file.Name {
		// If there's no package documentation, just use current file.
		decl := pgf
		for _, pgf := range pkg.CompiledGoFiles() {
			if pgf.File.Doc != nil {
				decl = pgf
			}
		}
		pkgRng, err := pgf.NodeMappedRange(file.Name)
		if err != nil {
			return nil, err
		}
		declRng, err := decl.NodeMappedRange(decl.File.Name)
		if err != nil {
			return nil, err
		}
		return &IdentifierInfo{
			Name:        file.Name.Name,
			ident:       file.Name,
			MappedRange: pkgRng,
			pkg:         pkg,
			qf:          qf,
			Snapshot:    snapshot,
			Declaration: Declaration{
				node:        decl.File.Name,
				nodeFile:    decl,
				MappedRange: []protocol.MappedRange{declRng},
			},
		}, nil
	}

	result := &IdentifierInfo{
		Snapshot:  snapshot,
		qf:        qf,
		pkg:       pkg,
		ident:     ident,
		enclosing: searchForEnclosing(pkg.GetTypesInfo(), path),
	}

	result.Name = result.ident.Name
	var err error
	if result.MappedRange, err = posToMappedRange(ctx, snapshot, pkg, result.ident.Pos(), result.ident.End()); err != nil {
		return nil, err
	}

	result.Declaration.obj = pkg.GetTypesInfo().ObjectOf(result.ident)
	if result.Declaration.obj == nil {
		// If there was no types.Object for the declaration, there might be an
		// implicit local variable declaration in a type switch.
		if objs, typ := typeSwitchImplicits(pkg.GetTypesInfo(), path); len(objs) > 0 {
			// There is no types.Object for the declaration of an implicit local variable,
			// but all of the types.Objects associated with the usages of this variable can be
			// used to connect it back to the declaration.
			// Preserve the first of these objects and treat it as if it were the declaring object.
			result.Declaration.obj = objs[0]
			result.Declaration.typeSwitchImplicit = typ
		} else {
			// Probably a type error.
			return nil, fmt.Errorf("%w for ident %v", errNoObjectFound, result.Name)
		}
	}

	// Handle builtins separately.
	if result.Declaration.obj.Parent() == types.Universe {
		builtin, err := snapshot.BuiltinFile(ctx)
		if err != nil {
			return nil, err
		}
		builtinObj := builtin.File.Scope.Lookup(result.Name)
		if builtinObj == nil {
			return nil, fmt.Errorf("no builtin object for %s", result.Name)
		}
		decl, ok := builtinObj.Decl.(ast.Node)
		if !ok {
			return nil, fmt.Errorf("no declaration for %s", result.Name)
		}
		result.Declaration.node = decl
		result.Declaration.nodeFile = builtin
		if typeSpec, ok := decl.(*ast.TypeSpec); ok {
			// Find the GenDecl (which has the doc comments) for the TypeSpec.
			result.Declaration.fullDecl = findGenDecl(builtin.File, typeSpec)
		}

		// The builtin package isn't in the dependency graph, so the usual
		// utilities won't work here.
		rng, err := builtin.PosMappedRange(decl.Pos(), decl.Pos()+token.Pos(len(result.Name)))
		if err != nil {
			return nil, err
		}
		result.Declaration.MappedRange = append(result.Declaration.MappedRange, rng)
		return result, nil
	}

	// (error).Error is a special case of builtin. Lots of checks to confirm
	// that this is the builtin Error.
	if obj := result.Declaration.obj; obj.Parent() == nil && obj.Pkg() == nil && obj.Name() == "Error" {
		if _, ok := obj.Type().(*types.Signature); ok {
			builtin, err := snapshot.BuiltinFile(ctx)
			if err != nil {
				return nil, err
			}
			// Look up "error" and then navigate to its only method.
			// The Error method does not appear in the builtin package's scope.
			const errorName = "error"
			builtinObj := builtin.File.Scope.Lookup(errorName)
			if builtinObj == nil {
				return nil, fmt.Errorf("no builtin object for %s", errorName)
			}
			decl, ok := builtinObj.Decl.(ast.Node)
			if !ok {
				return nil, fmt.Errorf("no declaration for %s", errorName)
			}
			spec, ok := decl.(*ast.TypeSpec)
			if !ok {
				return nil, fmt.Errorf("no type spec for %s", errorName)
			}
			iface, ok := spec.Type.(*ast.InterfaceType)
			if !ok {
				return nil, fmt.Errorf("%s is not an interface", errorName)
			}
			if iface.Methods.NumFields() != 1 {
				return nil, fmt.Errorf("expected 1 method for %s, got %v", errorName, iface.Methods.NumFields())
			}
			method := iface.Methods.List[0]
			if len(method.Names) != 1 {
				return nil, fmt.Errorf("expected 1 name for %v, got %v", method, len(method.Names))
			}
			name := method.Names[0].Name
			result.Declaration.node = method
			result.Declaration.nodeFile = builtin
			rng, err := builtin.PosMappedRange(method.Pos(), method.Pos()+token.Pos(len(name)))
			if err != nil {
				return nil, err
			}
			result.Declaration.MappedRange = append(result.Declaration.MappedRange, rng)
			return result, nil
		}
	}

	// If the original position was an embedded field, we want to jump
	// to the field's type definition, not the field's definition.
	if v, ok := result.Declaration.obj.(*types.Var); ok && v.Embedded() {
		// types.Info.Uses contains the embedded field's *types.TypeName.
		if typeName := pkg.GetTypesInfo().Uses[ident]; typeName != nil {
			result.Declaration.obj = typeName
		}
	}

	// TODO(adonovan): this step calls the somewhat expensive
	// findFileInDeps, which is also called below.  Refactor
	// objToMappedRange to separate the find-file from the
	// lookup-position steps to avoid the redundancy.
	obj := result.Declaration.obj
	rng, err := posToMappedRange(ctx, snapshot, pkg, obj.Pos(), adjustedObjEnd(obj))
	if err != nil {
		return nil, err
	}
	result.Declaration.MappedRange = append(result.Declaration.MappedRange, rng)

	declPos := result.Declaration.obj.Pos()
	objURI := span.URIFromPath(pkg.FileSet().File(declPos).Name())
	declFile, declPkg, err := findFileInDeps(ctx, snapshot, pkg, objURI)
	if err != nil {
		return nil, err
	}
	result.Declaration.node, _, _ = FindDeclInfo([]*ast.File{declFile.File}, declPos) // may be nil
	result.Declaration.nodeFile = declFile

	// Ensure that we have the full declaration, in case the declaration was
	// parsed in ParseExported and therefore could be missing information.
	if result.Declaration.fullDecl, err = fullNode(pkg.FileSet(), result.Declaration.obj, declPkg); err != nil {
		return nil, err
	}
	typ := pkg.GetTypesInfo().TypeOf(result.ident)
	if typ == nil {
		return result, nil
	}

	result.Inferred = inferredSignature(pkg.GetTypesInfo(), ident)

	result.Type.Object = typeToObject(typ)
	if result.Type.Object != nil {
		// Identifiers with the type "error" are a special case with no position.
		if hasErrorType(result.Type.Object) {
			return result, nil
		}
		obj := result.Type.Object
		// TODO(rfindley): no need to use an adjusted end here.
		if result.Type.MappedRange, err = posToMappedRange(ctx, snapshot, pkg, obj.Pos(), adjustedObjEnd(obj)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// findGenDecl determines the parent ast.GenDecl for a given ast.Spec.
func findGenDecl(f *ast.File, spec ast.Spec) *ast.GenDecl {
	for _, decl := range f.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			if genDecl.Pos() <= spec.Pos() && genDecl.End() >= spec.End() {
				return genDecl
			}
		}
	}
	return nil
}

// fullNode tries to extract the full spec corresponding to obj's declaration.
// If the package was not parsed in full, the declaration file will be
// re-parsed to ensure it has complete syntax.
func fullNode(fset *token.FileSet, obj types.Object, pkg Package) (ast.Decl, error) {
	// declaration in a different package... make sure we have full AST information.
	tok := fset.File(obj.Pos())
	uri := span.URIFromPath(tok.Name())
	pgf, err := pkg.File(uri)
	if err != nil {
		return nil, err
	}
	file := pgf.File
	pos := obj.Pos()
	if pgf.Mode != ParseFull {
		file2, _ := parser.ParseFile(fset, tok.Name(), pgf.Src, parser.AllErrors|parser.ParseComments)
		if file2 != nil {
			offset, err := safetoken.Offset(tok, obj.Pos())
			if err != nil {
				return nil, err
			}
			file = file2
			tok2 := fset.File(file2.Pos())
			pos = tok2.Pos(offset)
		}
	}
	path, _ := astutil.PathEnclosingInterval(file, pos, pos)
	for _, n := range path {
		if decl, ok := n.(ast.Decl); ok {
			return decl, nil
		}
	}
	return nil, nil
}

// inferredSignature determines the resolved non-generic signature for an
// identifier in an instantiation expression.
//
// If no such signature exists, it returns nil.
func inferredSignature(info *types.Info, id *ast.Ident) *types.Signature {
	inst := typeparams.GetInstances(info)[id]
	sig, _ := inst.Type.(*types.Signature)
	return sig
}

func searchForEnclosing(info *types.Info, path []ast.Node) *types.TypeName {
	for _, n := range path {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if sel, ok := info.Selections[n]; ok {
				recv := Deref(sel.Recv())

				// Keep track of the last exported type seen.
				var exported *types.TypeName
				if named, ok := recv.(*types.Named); ok && named.Obj().Exported() {
					exported = named.Obj()
				}
				// We don't want the last element, as that's the field or
				// method itself.
				for _, index := range sel.Index()[:len(sel.Index())-1] {
					if r, ok := recv.Underlying().(*types.Struct); ok {
						recv = Deref(r.Field(index).Type())
						if named, ok := recv.(*types.Named); ok && named.Obj().Exported() {
							exported = named.Obj()
						}
					}
				}
				return exported
			}
		case *ast.CompositeLit:
			if t, ok := info.Types[n]; ok {
				if named, _ := t.Type.(*types.Named); named != nil {
					return named.Obj()
				}
			}
		case *ast.TypeSpec:
			if _, ok := n.Type.(*ast.StructType); ok {
				if t, ok := info.Defs[n.Name]; ok {
					if tname, _ := t.(*types.TypeName); tname != nil {
						return tname
					}
				}
			}
		}
	}
	return nil
}

// typeToObject returns the relevant type name for the given type, after
// unwrapping pointers, arrays, slices, channels, and function signatures with
// a single non-error result.
func typeToObject(typ types.Type) *types.TypeName {
	switch typ := typ.(type) {
	case *types.Named:
		return typ.Obj()
	case *types.Pointer:
		return typeToObject(typ.Elem())
	case *types.Array:
		return typeToObject(typ.Elem())
	case *types.Slice:
		return typeToObject(typ.Elem())
	case *types.Chan:
		return typeToObject(typ.Elem())
	case *types.Signature:
		// Try to find a return value of a named type. If there's only one
		// such value, jump to its type definition.
		var res *types.TypeName

		results := typ.Results()
		for i := 0; i < results.Len(); i++ {
			obj := typeToObject(results.At(i).Type())
			if obj == nil || hasErrorType(obj) {
				// Skip builtins.
				continue
			}
			if res != nil {
				// The function/method must have only one return value of a named type.
				return nil
			}

			res = obj
		}
		return res
	default:
		return nil
	}
}

func hasErrorType(obj types.Object) bool {
	return types.IsInterface(obj.Type()) && obj.Pkg() == nil && obj.Name() == "error"
}

// importSpec handles positions inside of an *ast.ImportSpec.
func importSpec(ctx context.Context, snapshot Snapshot, pkg Package, pgf *ParsedGoFile, pos token.Pos) (*IdentifierInfo, error) {
	var imp *ast.ImportSpec
	for _, spec := range pgf.File.Imports {
		if spec.Path.Pos() <= pos && pos < spec.Path.End() {
			imp = spec
		}
	}
	if imp == nil {
		return nil, nil
	}
	importPath, err := strconv.Unquote(imp.Path.Value)
	if err != nil {
		return nil, fmt.Errorf("import path not quoted: %s (%v)", imp.Path.Value, err)
	}

	result := &IdentifierInfo{
		Snapshot: snapshot,
		Name:     importPath, // should this perhaps be imported.PkgPath()?
		pkg:      pkg,
	}
	if result.MappedRange, err = posToMappedRange(ctx, snapshot, pkg, imp.Path.Pos(), imp.Path.End()); err != nil {
		return nil, err
	}

	impID := pkg.Metadata().DepsByImpPath[ImportPath(importPath)]
	if impID == "" {
		return nil, fmt.Errorf("failed to resolve import %q", importPath)
	}
	impMetadata := snapshot.Metadata(impID)
	if impMetadata == nil {
		return nil, fmt.Errorf("failed to resolve import ID %q", impID)
	}
	for _, f := range impMetadata.CompiledGoFiles {
		fh, err := snapshot.GetFile(ctx, f)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		pgf, err := snapshot.ParseGo(ctx, fh, ParseHeader)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		rng, err := pgf.NodeMappedRange(pgf.File)
		if err != nil {
			return nil, err
		}
		result.Declaration.MappedRange = append(result.Declaration.MappedRange, rng)
	}

	result.Declaration.node = imp
	result.Declaration.nodeFile = pgf
	return result, nil
}

// typeSwitchImplicits returns all the implicit type switch objects that
// correspond to the leaf *ast.Ident. It also returns the original type
// associated with the identifier (outside of a case clause).
func typeSwitchImplicits(info *types.Info, path []ast.Node) ([]types.Object, types.Type) {
	ident, _ := path[0].(*ast.Ident)
	if ident == nil {
		return nil, nil
	}

	var (
		ts     *ast.TypeSwitchStmt
		assign *ast.AssignStmt
		cc     *ast.CaseClause
		obj    = info.ObjectOf(ident)
	)

	// Walk our ancestors to determine if our leaf ident refers to a
	// type switch variable, e.g. the "a" from "switch a := b.(type)".
Outer:
	for i := 1; i < len(path); i++ {
		switch n := path[i].(type) {
		case *ast.AssignStmt:
			// Check if ident is the "a" in "a := foo.(type)". The "a" in
			// this case has no types.Object, so check for ident equality.
			if len(n.Lhs) == 1 && n.Lhs[0] == ident {
				assign = n
			}
		case *ast.CaseClause:
			// Check if ident is a use of "a" within a case clause. Each
			// case clause implicitly maps "a" to a different types.Object,
			// so check if ident's object is the case clause's implicit
			// object.
			if obj != nil && info.Implicits[n] == obj {
				cc = n
			}
		case *ast.TypeSwitchStmt:
			// Look for the type switch that owns our previously found
			// *ast.AssignStmt or *ast.CaseClause.
			if n.Assign == assign {
				ts = n
				break Outer
			}

			for _, stmt := range n.Body.List {
				if stmt == cc {
					ts = n
					break Outer
				}
			}
		}
	}
	if ts == nil {
		return nil, nil
	}
	// Our leaf ident refers to a type switch variable. Fan out to the
	// type switch's implicit case clause objects.
	var objs []types.Object
	for _, cc := range ts.Body.List {
		if ccObj := info.Implicits[cc]; ccObj != nil {
			objs = append(objs, ccObj)
		}
	}
	// The right-hand side of a type switch should only have one
	// element, and we need to track its type in order to generate
	// hover information for implicit type switch variables.
	var typ types.Type
	if assign, ok := ts.Assign.(*ast.AssignStmt); ok && len(assign.Rhs) == 1 {
		if rhs := assign.Rhs[0].(*ast.TypeAssertExpr); ok {
			typ = info.TypeOf(rhs.X)
		}
	}
	return objs, typ
}
