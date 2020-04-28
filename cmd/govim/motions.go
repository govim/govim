package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"unicode/utf8"
)

func (v *vimstate) motion(args ...json.RawMessage) (interface{}, error) {
	// GOVIMMotion has the signature:
	//
	//     func GOVIMMotion(direction, target string)
	//
	// direction is either "previous" or "next" (relative to
	// the cursor position). target is based as closely as possible on the
	// definitions in go/ast.
	//
	// For example the call GOVIMMotion("next", "File.Decls.End()") moves the
	// cursor to the first File Decl end position after the current cursor
	// position.

	if len(args) != 2 {
		return nil, fmt.Errorf("expected two string args")
	}
	var strargs []string
	for i, a := range args {
		// We explicitly attempt to parse a string here because it's a Govim
		// level (user) error for the type of the parameters to be wrong.
		var s string
		if err := json.Unmarshal(a, &s); err != nil {
			return nil, fmt.Errorf("failed to parse argument %v as a string: %v", i+1, err)
		}
		strargs = append(strargs, s)
	}

	// Get the current cursor position
	b, point, err := v.bufCursorPos()
	if err != nil {
		return nil, fmt.Errorf("failed to get current position: %v", err)
	}

	// Now ensure we block for the result of any in-flight parse
	if b.ASTWait == nil {
		return nil, fmt.Errorf("got motion request before buffer had loaded?")
	}
	<-b.ASTWait

	var file *token.File
	b.Fset.Iterate(func(f *token.File) bool {
		if f.Name() == b.Name {
			file = f
			return false
		}
		panic(fmt.Errorf("expected to find a single file in the fset"))
	})

	pos := file.Pos(point.Offset())
	if !pos.IsValid() {
		// Nothing we can do here.
		return nil, nil
	}

	// Now figure out where the user wants us to move the cursor
	dir := strargs[0]
	target := strargs[1]

	switch dir {
	case "next", "prev":
	default:
		return nil, fmt.Errorf("got unknown direction %q", dir)
	}
	var resolv func(n ast.Node) token.Pos
	switch target {
	case "File.Decls.End()":
		resolv = func(n ast.Node) token.Pos {
			// The user sees themselves as being at the end when the cursor is
			// before the closing brace, not after. Hence adjust backwards
			tf := b.Fset.File(pos)
			tfe := token.Pos(tf.Base() + tf.Size())
			if n.End() > tfe {
				// Work around https://github.com/golang/go/issues/33649
				return tfe
			}
			offset := b.Fset.File(pos).Offset(n.End())
			_, size := utf8.DecodeLastRune(b.Contents()[:offset])
			return b.Fset.File(pos).Pos(offset - size)
		}
	case "File.Decls.Pos()":
		resolv = func(n ast.Node) token.Pos {
			return n.Pos()
		}
	default:
		return nil, fmt.Errorf("got unknown target %q", target)
	}

	// Brute force this for now
	var targetNode ast.Node
	switch dir {
	case "next":
		for i := 0; i < len(b.AST.Decls); i++ {
			d := b.AST.Decls[i]
			resolved := resolv(d)
			if !resolved.IsValid() {
				// We can't complete the motion because of an invalid target position.
				// Likely a result of a syntax error. In future we could improve this
				// by giving a non-fatal warning message, just silently "fail" for now.
				return nil, nil
			}
			if resolved > pos {
				targetNode = d
				break
			}
		}
	case "prev":
		for i := len(b.AST.Decls) - 1; i >= 0; i-- {
			d := b.AST.Decls[i]
			resolved := resolv(d)
			if !resolved.IsValid() {
				// We can't complete the motion because of an invalid target position.
				// Likely a result of a syntax error. In future we could improve this
				// by giving a non-fatal warning message, just silently "fail" for now.
				return nil, nil
			}
			if resolved < pos {
				targetNode = d
				break
			}
		}
	}

	if targetNode != nil {
		position := b.Fset.Position(resolv(targetNode))
		v.ChannelCall("cursor", position.Line, position.Column)
	}
	return nil, nil
}
