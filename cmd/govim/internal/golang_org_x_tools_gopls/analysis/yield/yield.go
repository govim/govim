// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package yield

// TODO(adonovan): also check for this pattern:
//
// 	for x := range seq {
// 		yield(x)
// 	}
//
// which should be entirely rewritten as
//
// 	seq(yield)
//
// to avoid unnecesary range desugaring and chains of dynamic calls.

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/ssa"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/util/safetoken"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/analysisinternal"
)

//go:embed doc.go
var doc string

var Analyzer = &analysis.Analyzer{
	Name:     "yield",
	Doc:      analysisinternal.MustExtractDoc(doc, "yield"),
	Requires: []*analysis.Analyzer{inspect.Analyzer, buildssa.Analyzer},
	Run:      run,
	URL:      "https://pkg.go.dev/github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/analysis/yield",
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspector := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Find all calls to yield of the right type.
	yieldCalls := make(map[token.Pos]*ast.CallExpr) // keyed by CallExpr.Lparen.
	nodeFilter := []ast.Node{(*ast.CallExpr)(nil)}
	inspector.Preorder(nodeFilter, func(n ast.Node) {
		call := n.(*ast.CallExpr)
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "yield" {
			if sig, ok := pass.TypesInfo.TypeOf(id).(*types.Signature); ok &&
				sig.Params().Len() < 3 &&
				sig.Results().Len() == 1 &&
				types.Identical(sig.Results().At(0).Type(), types.Typ[types.Bool]) {
				yieldCalls[call.Lparen] = call
			}
		}
	})

	// Common case: nothing to do.
	if len(yieldCalls) == 0 {
		return nil, nil
	}

	// Study the control flow using SSA.
	buildssa := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	for _, fn := range buildssa.SrcFuncs {
		// TODO(adonovan): opt: skip functions that don't contain any yield calls.

		// Find the yield calls in SSA.
		type callInfo struct {
			syntax   *ast.CallExpr
			index    int // index of instruction within its block
			reported bool
		}
		ssaYieldCalls := make(map[*ssa.Call]*callInfo)
		for _, b := range fn.Blocks {
			for i, instr := range b.Instrs {
				if call, ok := instr.(*ssa.Call); ok {
					if syntax, ok := yieldCalls[call.Pos()]; ok {
						ssaYieldCalls[call] = &callInfo{syntax: syntax, index: i}
					}
				}
			}
		}

		// Now search for a control path from the instruction after a
		// yield call to another yield call--possible the same one,
		// following all block successors except "if yield() { ... }";
		// in such cases we know that yield returned true.
		for call, info := range ssaYieldCalls {
			visited := make([]bool, len(fn.Blocks)) // visited BasicBlock.Indexes

			// visit visits the instructions of a block (or a suffix if start > 0).
			var visit func(b *ssa.BasicBlock, start int)
			visit = func(b *ssa.BasicBlock, start int) {
				if !visited[b.Index] {
					if start == 0 {
						visited[b.Index] = true
					}
					for _, instr := range b.Instrs[start:] {
						switch instr := instr.(type) {
						case *ssa.Call:
							if !info.reported && ssaYieldCalls[instr] != nil {
								info.reported = true
								where := "" // "" => same yield call (a loop)
								if instr != call {
									otherLine := safetoken.StartPosition(pass.Fset, instr.Pos()).Line
									where = fmt.Sprintf("(on L%d) ", otherLine)
								}
								pass.Reportf(call.Pos(), "yield may be called again %safter returning false", where)
							}
						case *ssa.If:
							// Visit both successors, unless cond is yield() or its negation.
							// In that case visit only the "if !yield()" block.
							cond := instr.Cond
							t, f := b.Succs[0], b.Succs[1]
							if unop, ok := cond.(*ssa.UnOp); ok && unop.Op == token.NOT {
								cond, t, f = unop.X, f, t
							}
							if cond, ok := cond.(*ssa.Call); ok && ssaYieldCalls[cond] != nil {
								// Skip the successor reached by "if yield() { ... }".
							} else {
								visit(t, 0)
							}
							visit(f, 0)

						case *ssa.Jump:
							visit(b.Succs[0], 0)
						}
					}
				}
			}

			// Start at the instruction after the yield call.
			visit(call.Block(), info.index+1)
		}
	}

	return nil, nil
}
