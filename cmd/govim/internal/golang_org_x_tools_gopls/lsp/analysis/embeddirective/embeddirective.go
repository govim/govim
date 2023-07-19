// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package embeddirective defines an Analyzer that validates import for //go:embed directive.
package embeddirective

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const Doc = `check for //go:embed directive import

This analyzer checks that the embed package is imported when source code contains //go:embed comment directives.
The embed package must be imported for //go:embed directives to function.import _ "embed".`

var Analyzer = &analysis.Analyzer{
	Name:             "embed",
	Doc:              Doc,
	Requires:         []*analysis.Analyzer{},
	Run:              run,
	RunDespiteErrors: true,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		comments := embedDirectiveComments(f)
		if len(comments) == 0 {
			continue // nothing to check
		}

		hasEmbedImport := false
		for _, imp := range f.Imports {
			if imp.Path.Value == `"embed"` {
				hasEmbedImport = true
				break
			}
		}

		for _, c := range comments {
			report := func(msg string) {
				pass.Report(analysis.Diagnostic{
					Pos:     c.Pos(),
					End:     c.Pos() + token.Pos(len("//go:embed")),
					Message: msg,
				})
			}

			if !hasEmbedImport {
				report(`must import "embed" when using go:embed directives`)
			}

			spec := nextVarSpec(c, f)
			switch {
			case spec == nil:
				report(`go:embed directives must precede a "var" declaration`)
			case len(spec.Names) > 1:
				report("declarations following go:embed directives must define a single variable")
			case len(spec.Values) > 0:
				report("declarations following go:embed directives must not specify a value")
			}
		}
	}
	return nil, nil
}

// embedDirectiveComments returns all comments in f that contains a //go:embed directive.
func embedDirectiveComments(f *ast.File) []*ast.Comment {
	comments := []*ast.Comment{}
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "//go:embed ") {
				comments = append(comments, c)
			}
		}
	}
	return comments
}

// nextVarSpec returns the ValueSpec for the variable declaration immediately following
// the go:embed comment, or nil if the next declaration is not a variable declaration.
func nextVarSpec(com *ast.Comment, f *ast.File) *ast.ValueSpec {
	// Embed directives must be followed by a declaration of one variable with no value.
	// There may be comments and empty lines between the directive and the declaration.
	var nextDecl ast.Decl
	for _, d := range f.Decls {
		if com.End() < d.End() {
			nextDecl = d
			break
		}
	}
	if nextDecl == nil || nextDecl.Pos() == token.NoPos {
		return nil
	}
	decl, ok := nextDecl.(*ast.GenDecl)
	if !ok {
		return nil
	}
	if decl.Tok != token.VAR {
		return nil
	}

	// var declarations can be both freestanding and blocks (with parenthesis).
	// Only the first variable spec following the directive is interesting.
	var nextSpec ast.Spec
	for _, s := range decl.Specs {
		if com.End() < s.End() {
			nextSpec = s
			break
		}
	}
	if nextSpec == nil {
		return nil
	}
	spec, ok := nextSpec.(*ast.ValueSpec)
	if !ok {
		// Invalid AST, but keep going.
		return nil
	}
	return spec
}
