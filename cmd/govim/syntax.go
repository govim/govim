package main

import (
	"go/ast"
	"go/token"

	"github.com/myitcv/govim"
)

type matchaddposDict struct {
	Window int `json:"window"`
}

func (v *vimstate) highlight() error {
	return v.highlightViewport(v.Viewport())
}

func (v *vimstate) highlightViewport(vp govim.Viewport) error {
	v.ChannelEx("set lazyredraw")

	for _, w := range vp.Windows {
		b := v.buffers[w.BufNr]

		// TODO this is a bit gross; if we don't have a buffer we'll get an event
		// later that does. Continue for now. Feels like we should be more
		// concrete here
		if b == nil {
			continue
		}

		matches := v.winHighlihts[w.WinID]
		if matches == nil {
			matches = make(map[position]*match)
			v.winHighlihts[w.WinID] = matches
		}

		sg := &synGenerator{
			fset:      b.Fset,
			nodes:     matches,
			LineStart: w.TopLine,
			LineEnd:   w.TopLine + w.Height,
		}

		// TODO - fix this
		if b.AST == nil {
			continue
		}

		// generate our highlight positions
		ast.Walk(sg, b.AST)
		for _, c := range b.AST.Comments {
			ast.Walk(sg, c)
		}

		for pos, m := range matches {
			switch m.a {
			case ActionAdd:
				m.winid = w.WinID
				m.id = v.ParseUint(v.ChannelCall("matchaddpos", pos.t.String(), [][3]int{{pos.line, pos.col, pos.l}}, 0, -1, matchaddposDict{Window: w.WinID}))
				m.a = ActionDelete
			case ActionDelete:
				if m.id == 0 || m.winid != vp.Current.WinID {
					continue
				}
				v.ChannelCall("matchdelete", m.id, m.winid)
				delete(matches, pos)
			case ActionKeep:
				m.a = ActionDelete
			}
		}
	}

	v.ChannelEx("set nolazyredraw")

	return nil
}

type synGenerator struct {
	fset      *token.FileSet
	nodes     map[position]*match
	LineStart int
	LineEnd   int
}

type position struct {
	l    int
	line int
	col  int
	t    nodetype
}

type action uint32

type nodetype uint32

const (
	ActionAdd action = iota
	ActionKeep
	ActionDelete
)

//go:generate go run golang.org/x/tools/cmd/stringer -type=nodetype -linecomment -output gen_nodetype_stringer.go

const (
	NodeTypeKeyword     nodetype = iota // Keyword
	NodeTypeStatement                   // Statement
	NodeTypeString                      // String
	NodeTypeType                        // Type
	NodeTypeConditional                 // Conditional
	NodeTypeFunction                    // Function
	NodeTypeComment                     // Comment
	NodeTypeLabel                       // Label
	NodeTypeRepeat                      // Repeat
)

type match struct {
	id    uint
	winid int
	a     action
}

func (s *synGenerator) addNode(t nodetype, l int, _p token.Pos) {
	p := s.fset.Position(_p)

	if p.Line < s.LineStart || p.Line > s.LineEnd {
		return
	}

	pos := position{t: t, l: l, line: p.Line, col: p.Column}
	if m, ok := s.nodes[pos]; ok {
		m.a = ActionKeep
	} else {
		s.nodes[pos] = &match{a: ActionAdd}
	}
}

func (s *synGenerator) Visit(node ast.Node) ast.Visitor {
	var handleType func(ast.Expr)
	handleType = func(t ast.Expr) {
		switch node := t.(type) {
		case *ast.Ident:
			s.addNode(NodeTypeType, len(node.Name), node.NamePos)
		case *ast.FuncType:
			s.addNode(NodeTypeKeyword, 4, node.Func)
		case *ast.ChanType:
			s.addNode(NodeTypeType, 4, node.Begin)
			handleType(node.Value)
		case *ast.MapType:
			s.addNode(NodeTypeType, 3, node.Map)
			handleType(node.Key)
			handleType(node.Value)
		}
	}
	switch node := node.(type) {
	case *ast.File:
		s.addNode(NodeTypeStatement, 7, node.Package)
	case *ast.BasicLit:
		if node.Kind == token.STRING {
			s.addNode(NodeTypeString, len(node.Value), node.ValuePos)
		}
	case *ast.Comment:
		s.addNode(NodeTypeComment, len(node.Text), node.Slash)
	case *ast.GenDecl:
		switch node.Tok {
		case token.VAR:
			s.addNode(NodeTypeKeyword, 3, node.TokPos)
		case token.IMPORT:
			s.addNode(NodeTypeStatement, 6, node.TokPos)
		case token.CONST:
			s.addNode(NodeTypeKeyword, 5, node.TokPos)
		case token.TYPE:
			s.addNode(NodeTypeKeyword, 4, node.TokPos)
		}
	case *ast.StructType:
		s.addNode(NodeTypeKeyword, 6, node.Struct)
	case *ast.InterfaceType:
		s.addNode(NodeTypeKeyword, 9, node.Interface)
	case *ast.ReturnStmt:
		s.addNode(NodeTypeKeyword, 6, node.Return)
	case *ast.BranchStmt:
		s.addNode(NodeTypeKeyword, len(node.Tok.String()), node.TokPos)
	case *ast.ForStmt:
		s.addNode(NodeTypeRepeat, 3, node.For)
	case *ast.GoStmt:
		s.addNode(NodeTypeStatement, 2, node.Go)
	case *ast.DeferStmt:
		s.addNode(NodeTypeStatement, 5, node.Defer)
	case *ast.FuncDecl:
		s.addNode(NodeTypeFunction, len(node.Name.Name), node.Name.NamePos)
		handleType(node.Type)
	case *ast.Field:
		handleType(node.Type)
	case *ast.ValueSpec:
		handleType(node.Type)
	case *ast.SwitchStmt:
		s.addNode(NodeTypeConditional, 6, node.Switch)
	case *ast.SelectStmt:
		s.addNode(NodeTypeConditional, 6, node.Select)
	case *ast.CaseClause:
		s.addNode(NodeTypeLabel, 4, node.Case)
	case *ast.RangeStmt:
		s.addNode(NodeTypeRepeat, 3, node.For)
		key := node.Key.(*ast.Ident)
		ass := key.Obj.Decl.(*ast.AssignStmt)
		rhs := ass.Rhs[0].(*ast.UnaryExpr)
		s.addNode(NodeTypeRepeat, 5, rhs.OpPos)
	case *ast.IfStmt:
		s.addNode(NodeTypeConditional, 2, node.If)
	}
	return s
}
