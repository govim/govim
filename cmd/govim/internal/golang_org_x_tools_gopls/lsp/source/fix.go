// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/bug"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/analysis/embeddirective"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/analysis/fillstruct"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/analysis/undeclaredname"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/span"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/imports"
)

type (
	// SuggestedFixFunc is a function used to get the suggested fixes for a given
	// gopls command, some of which are provided by go/analysis.Analyzers. Some of
	// the analyzers in internal/lsp/analysis are not efficient enough to include
	// suggested fixes with their diagnostics, so we have to compute them
	// separately. Such analyzers should provide a function with a signature of
	// SuggestedFixFunc.
	//
	// The returned FileSet must map all token.Pos found in the suggested text
	// edits.
	SuggestedFixFunc  func(ctx context.Context, snapshot Snapshot, fh FileHandle, pRng protocol.Range) (*token.FileSet, *analysis.SuggestedFix, error)
	singleFileFixFunc func(fset *token.FileSet, start, end token.Pos, src []byte, file *ast.File, pkg *types.Package, info *types.Info) (*analysis.SuggestedFix, error)
)

// These strings identify kinds of suggested fix, both in Analyzer.Fix
// and in the ApplyFix subcommand (see ExecuteCommand and ApplyFixArgs.Fix).
const (
	FillStruct        = "fill_struct"
	StubMethods       = "stub_methods"
	UndeclaredName    = "undeclared_name"
	ExtractVariable   = "extract_variable"
	ExtractFunction   = "extract_function"
	ExtractMethod     = "extract_method"
	InlineCall        = "inline_call"
	InvertIfCondition = "invert_if_condition"
	AddEmbedImport    = "add_embed_import"
)

// suggestedFixes maps a suggested fix command id to its handler.
var suggestedFixes = map[string]SuggestedFixFunc{
	FillStruct:        singleFile(fillstruct.SuggestedFix),
	UndeclaredName:    singleFile(undeclaredname.SuggestedFix),
	ExtractVariable:   singleFile(extractVariable),
	InlineCall:        inlineCall,
	ExtractFunction:   singleFile(extractFunction),
	ExtractMethod:     singleFile(extractMethod),
	InvertIfCondition: singleFile(invertIfCondition),
	StubMethods:       stubSuggestedFixFunc,
	AddEmbedImport:    addEmbedImport,
}

// singleFile calls analyzers that expect inputs for a single file
func singleFile(sf singleFileFixFunc) SuggestedFixFunc {
	return func(ctx context.Context, snapshot Snapshot, fh FileHandle, pRng protocol.Range) (*token.FileSet, *analysis.SuggestedFix, error) {
		pkg, pgf, err := NarrowestPackageForFile(ctx, snapshot, fh.URI())
		if err != nil {
			return nil, nil, err
		}
		start, end, err := pgf.RangePos(pRng)
		if err != nil {
			return nil, nil, err
		}
		fix, err := sf(pkg.FileSet(), start, end, pgf.Src, pgf.File, pkg.GetTypes(), pkg.GetTypesInfo())
		return pkg.FileSet(), fix, err
	}
}

func SuggestedFixFromCommand(cmd protocol.Command, kind protocol.CodeActionKind) SuggestedFix {
	return SuggestedFix{
		Title:      cmd.Title,
		Command:    &cmd,
		ActionKind: kind,
	}
}

// ApplyFix applies the command's suggested fix to the given file and
// range, returning the resulting edits.
func ApplyFix(ctx context.Context, fix string, snapshot Snapshot, fh FileHandle, pRng protocol.Range) ([]protocol.TextDocumentEdit, error) {
	handler, ok := suggestedFixes[fix]
	if !ok {
		return nil, fmt.Errorf("no suggested fix function for %s", fix)
	}
	fset, suggestion, err := handler(ctx, snapshot, fh, pRng)
	if err != nil {
		return nil, err
	}
	if suggestion == nil {
		return nil, nil
	}
	editsPerFile := map[span.URI]*protocol.TextDocumentEdit{}
	for _, edit := range suggestion.TextEdits {
		tokFile := fset.File(edit.Pos)
		if tokFile == nil {
			return nil, bug.Errorf("no file for edit position")
		}
		end := edit.End
		if !end.IsValid() {
			end = edit.Pos
		}
		fh, err := snapshot.ReadFile(ctx, span.URIFromPath(tokFile.Name()))
		if err != nil {
			return nil, err
		}
		te, ok := editsPerFile[fh.URI()]
		if !ok {
			te = &protocol.TextDocumentEdit{
				TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
					Version: fh.Version(),
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{
						URI: protocol.URIFromSpanURI(fh.URI()),
					},
				},
			}
			editsPerFile[fh.URI()] = te
		}
		content, err := fh.Content()
		if err != nil {
			return nil, err
		}
		m := protocol.NewMapper(fh.URI(), content)
		rng, err := m.PosRange(tokFile, edit.Pos, end)
		if err != nil {
			return nil, err
		}
		te.Edits = append(te.Edits, protocol.TextEdit{
			Range:   rng,
			NewText: string(edit.NewText),
		})
	}
	var edits []protocol.TextDocumentEdit
	for _, edit := range editsPerFile {
		edits = append(edits, *edit)
	}
	return edits, nil
}

// fixedByImportingEmbed returns true if diag can be fixed by addEmbedImport.
func fixedByImportingEmbed(diag *Diagnostic) bool {
	if diag == nil {
		return false
	}
	return diag.Message == embeddirective.MissingImportMessage
}

// addEmbedImport adds a missing embed "embed" import with blank name.
func addEmbedImport(ctx context.Context, snapshot Snapshot, fh FileHandle, _ protocol.Range) (*token.FileSet, *analysis.SuggestedFix, error) {
	pkg, pgf, err := NarrowestPackageForFile(ctx, snapshot, fh.URI())
	if err != nil {
		return nil, nil, fmt.Errorf("narrow pkg: %w", err)
	}

	// Like source.AddImport, but with _ as Name and using our pgf.
	protoEdits, err := ComputeOneImportFixEdits(snapshot, pgf, &imports.ImportFix{
		StmtInfo: imports.ImportInfo{
			ImportPath: "embed",
			Name:       "_",
		},
		FixType: imports.AddImport,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("compute edits: %w", err)
	}

	var edits []analysis.TextEdit
	for _, e := range protoEdits {
		start, end, err := pgf.RangePos(e.Range)
		if err != nil {
			return nil, nil, fmt.Errorf("map range: %w", err)
		}
		edits = append(edits, analysis.TextEdit{
			Pos:     start,
			End:     end,
			NewText: []byte(e.NewText),
		})
	}

	fix := &analysis.SuggestedFix{
		Message:   "Add embed import",
		TextEdits: edits,
	}
	return pkg.FileSet(), fix, nil
}
