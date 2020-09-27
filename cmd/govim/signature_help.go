package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/types"
	"golang.org/x/tools/go/ast/astutil"
)

func (v *vimstate) signatureHelp(flags govim.CommandFlags, args ...string) error {
	b, p, err := v.bufCursorPos()
	if err != nil {
		return fmt.Errorf("failed to get current cursor position: %v", err)
	}
	params := &protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(b.URI()),
			},
			Position: p.ToPosition(),
		},
	}
	res, err := v.server.SignatureHelp(context.Background(), params)
	if err != nil {
		return fmt.Errorf("called to gopls.Completion failed: %v", err)
	}
	if res == nil || len(res.Signatures) == 0 {
		return nil
	}
	sigInx := int(res.ActiveSignature)
	if l := len(res.Signatures); sigInx >= l {
		return fmt.Errorf("active signature not in list (i: %d, len: %d)", sigInx, l)
	}
	sig := res.Signatures[sigInx]

	var activeParam string
	// According to LSP Specification 3.15 the server might send an active parameter index
	// that is outside the range of parameters sent so we need to ensure it exists here.
	if i := int(res.ActiveParameter); i < len(sig.Parameters) {
		activeParam = sig.Parameters[i].Label
	}

	// Use our locally parsed AST to find where to place this signature
	<-b.ASTWait
	var file *token.File
	b.Fset.Iterate(func(f *token.File) bool {
		if f.Name() == b.Name {
			file = f
			return false
		}
		panic(fmt.Errorf("expected to find a single file in the fset"))
	})
	pos := file.Pos(p.Offset())
	if !pos.IsValid() {
		return fmt.Errorf("failed to convert Vim point to Pos: %v", err)
	}
	var callExpr *ast.CallExpr
	path, _ := astutil.PathEnclosingInterval(b.AST, pos, pos)
	if path == nil {
		return fmt.Errorf("cannot find node enclosing position")
	}
FindCall:
	for _, node := range path {
		switch node := node.(type) {
		case *ast.CallExpr:
			if pos >= node.Lparen && pos <= node.Rparen {
				callExpr = node
				break FindCall
			}
		case *ast.FuncLit, *ast.FuncType:
			// The user is within an anonymous function,
			// which may be the parameter to the *ast.CallExpr.
			// Don't show signature help in this case.
			return fmt.Errorf("no signature help within a function declaration")
		}
	}
	if callExpr == nil || callExpr.Fun == nil {
		return fmt.Errorf("cannot find an enclosing function")
	}
	// If the *ast.CallExpr is based on an *ast.SelectorExpr then
	// the Pos() will be that of the X of the *ast.SelectorExpr.
	// gopls only returns the Sel part of such an *ast.SelectorExpr
	// hence we need to adjust accordingly
	var placePos token.Pos
	switch f := callExpr.Fun.(type) {
	case *ast.Ident, *ast.FuncLit:
		placePos = callExpr.Pos()
	case *ast.SelectorExpr:
		placePos = f.Sel.Pos()
	default:
		panic(fmt.Errorf("unknown case for %T", f))
	}
	placeOffset := file.Position(placePos).Offset
	placePoint, err := types.PointFromOffset(b, placeOffset)
	if err != nil {
		return fmt.Errorf("failed to convert place offset to Point: %v", err)
	}
	var screenPos struct {
		Row int `json:"row"`
		Col int `json:"col"`
	}
	v.Parse(v.ChannelCall("screenpos", p.WinID, placePoint.Line(), placePoint.Col()), &screenPos)

	opts := make(map[string]interface{})
	opts["moved"] = "any"
	opts["pos"] = "botleft"
	opts["padding"] = []int{0, 1, 0, 1}
	opts["wrap"] = false
	opts["line"] = screenPos.Row - 1
	opts["col"] = screenPos.Col - 1
	opts["close"] = "click"

	// formatPopupLine applies text properties to a signature help line and the active
	// parameter (if found).
	formatPopupLine := func(text, param string) types.PopupLine {
		sigProp := string(config.HighlightSignature)
		paramProp := string(config.HighlightSignatureParam)
		popupLine := types.PopupLine{
			Text:  text,
			Props: []types.PopupProp{{Type: sigProp, Col: 1, Len: len(text)}},
		}

		if i := strings.Index(text, param); param != "" && i >= 0 {
			popupLine.Props = append(popupLine.Props,
				types.PopupProp{Type: paramProp, Col: i + 1, Len: len(param)})
		}
		return popupLine
	}

	var lines []types.PopupLine
	for _, l := range strings.Split(sig.Label, "\n") {
		lines = append(lines, formatPopupLine(l, activeParam))
	}
	v.ChannelCall("popup_create", lines, opts)

	return nil
}
