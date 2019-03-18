package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

type highlighArgs struct {
	Buffer    string
	BufName   string
	BufNr     int
	LineStart int
	LineEnd   int
	ColStart  int
	ColEnd    int
}

func (d *driver) parseAndHighlight() error {
	var args highlighArgs
	d.Parse(d.ChannelExpr(`{`+
		`"Buffer": join(getline(0, "$"), "\n"),`+
		`"BufName": bufname("%"),`+
		`"BufNr": bufnr(bufname("%")),`+
		`"LineStart": winsaveview()['topline'],`+
		`"LineEnd": winsaveview()['topline'] + winheight('%'),`+
		`"ColStart": winsaveview()['leftcol'],`+
		`"ColEnd": winsaveview()['leftcol'] + winwidth('%')`+
		`}`), &args)

	if args.BufNr == -1 {
		return fmt.Errorf("got unknown buffer")
	}

	sg, ok := d.buffSyntax[args.BufNr]
	if !ok {
		sg = newSynGenerator(d)
		d.buffSyntax[args.BufNr] = sg
	}

	f, err := parser.ParseFile(sg.fset, args.BufName, args.Buffer, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return nil
	}

	sg.file = f
	sg.LineStart = args.LineStart
	sg.LineEnd = args.LineEnd
	sg.ColStart = args.ColStart
	sg.ColEnd = args.ColEnd

	// generate our highlight positions
	ast.Walk(sg, f)
	for _, c := range f.Comments {
		ast.Walk(sg, c)
	}

	// set the highlights
	sg.sweepMap()

	return nil
}

func (d *driver) highlight() error {
	var args highlighArgs
	d.Parse(d.ChannelExpr(`{`+
		`"BufName": bufname("%"),`+
		`"BufNr": bufnr(bufname("%")),`+
		`"LineStart": winsaveview()['topline'],`+
		`"LineEnd": winsaveview()['topline'] + winheight('%'),`+
		`"ColStart": winsaveview()['leftcol'],`+
		`"ColEnd": winsaveview()['leftcol'] + winwidth('%')`+
		`}`), &args)

	if args.BufNr == -1 {
		return fmt.Errorf("got unknown buffer")
	}

	sg, ok := d.buffSyntax[args.BufNr]
	if !ok {
		return d.parseAndHighlight()
	}

	sg.LineStart = args.LineStart
	sg.LineEnd = args.LineEnd
	sg.ColStart = args.ColStart
	sg.ColEnd = args.ColEnd

	// generate our highlight positions
	ast.Walk(sg, sg.file)
	for _, c := range sg.file.Comments {
		ast.Walk(sg, c)
	}

	// set the highlights
	sg.sweepMap()

	return nil
}

type synGenerator struct {
	fset  *token.FileSet
	nodes map[position]*match
	file  *ast.File

	*driver

	LineStart int
	LineEnd   int
	ColStart  int
	ColEnd    int
}

func newSynGenerator(d *driver) *synGenerator {
	return &synGenerator{
		driver: d,
		fset:   token.NewFileSet(),
		nodes:  make(map[position]*match),
	}
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

//go:generate gobin -m -run golang.org/x/tools/cmd/stringer -type=nodetype -linecomment -output gen_nodetype_stringer.go

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
	id uint
	a  action
}

func (s *synGenerator) sweepMap() {
	for pos, m := range s.nodes {
		switch m.a {
		case ActionAdd:
			m.id = s.ParseUint(s.ChannelCall("matchaddpos", pos.t.String(), [][3]int{{pos.line, pos.col, pos.l}}))
			m.a = ActionDelete
		case ActionDelete:
			if m.id == 0 {
				continue
			}
			s.ChannelCall("matchdelete", m.id)
			delete(s.nodes, pos)
		case ActionKeep:
			m.a = ActionDelete
		}
	}
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
