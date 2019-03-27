package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/russross/blackfriday/v2"
)

type Pos struct {
	Filename string
	Line     int
	Col      int
}

func (d *driver) cursorPos() (c Pos, err error) {
	var vp struct {
		Filename string `json:"filename"`
		Line     string `json:"line"`
		LineNr   int    `json:"linenr"`
		ColNr    int    `json:"colnr"`
	}
	expr := d.ChannelExpr(`{"filename": expand('%:p'), "line": getline("."), "linenr": line("."), "colnr": col(".")}`)
	if err = json.Unmarshal(expr, &vp); err != nil {
		return Pos{}, fmt.Errorf("failed to unmarshal current position info: %v", err)
	}
	var p = Pos{
		Filename: vp.Filename,
		Line:     vp.LineNr,
		Col:      byteToRuneOffset(vp.Line, vp.ColNr),
	}
	return p, nil
}

func (d *driver) mousePos() (c Pos, err error) {
	var vp struct {
		Filename string `json:"filename"`
		Line     string `json:"line"`
		LineNr   int    `json:"linenr"`
		ColNr    int    `json:"colnr"`
	}
	expr := d.ChannelExpr(`{"filename": fnamemodify(bufname(v:beval_bufnr), ":p"), "line": getbufline(v:beval_bufnr, v:beval_lnum)[0], "linenr": v:beval_lnum, "colnr": v:beval_col}`)
	if err = json.Unmarshal(expr, &vp); err != nil {
		return Pos{}, fmt.Errorf("failed to unmarshal current position info: %v", err)
	}
	var p = Pos{
		Filename: vp.Filename,
		Line:     vp.LineNr,
		Col:      byteToRuneOffset(vp.Line, vp.ColNr),
	}
	return p, nil
}

func byteToRuneOffset(s string, o int) (j int) {
	for i := range s {
		if i == o {
			return
		}
		j++
	}
	return -1
}

type plainMarkdown struct{}

var _ blackfriday.Renderer = plainMarkdown{}

func (p plainMarkdown) RenderNode(w io.Writer, node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
	fmt.Fprint(w, string(node.Literal))
	return blackfriday.GoToNext
}

func (p plainMarkdown) RenderHeader(w io.Writer, ast *blackfriday.Node) {}
func (p plainMarkdown) RenderFooter(w io.Writer, ast *blackfriday.Node) {}
