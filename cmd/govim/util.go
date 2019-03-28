package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"

	"github.com/russross/blackfriday/v2"
)

type Buffer struct {
	Num      int
	Name     string
	Contents []byte
	Version  int
}

func (d *driver) fetchCurrentBufferInfo() (buf Buffer, err error) {
	var b struct {
		Num      int
		Name     string
		Contents string
	}
	expr := d.ChannelExpr(`{"Num": bufnr(""), "Name": expand('%:p'), "Contents": join(getline(0, "$"), "\n")}`)
	if err = json.Unmarshal(expr, &b); err != nil {
		err = fmt.Errorf("failed to unmarshal current buffer info: %v", err)
	} else {
		buf.Num = b.Num
		buf.Name = b.Name
		buf.Contents = []byte(b.Contents)
	}
	return
}

type Pos struct {
	BufNum int `json:"bufnum"`
	Line   int `json:"line"`
	Col    int `json:"col"`
}

func (d *driver) cursorPos() (c Pos, err error) {
	expr := d.ChannelExpr(`{"bufnum": bufnr(""), "line": line("."), "col": col(".")}`)
	if err = json.Unmarshal(expr, &c); err != nil {
		err = fmt.Errorf("failed to unmarshal current cursor position info: %v", err)
	}
	return
}

func (d *driver) mousePos() (c Pos, err error) {
	expr := d.ChannelExpr(`{"bufnum": v:beval_bufnr, "line": v:beval_lnum, "col": v:beval_col}`)
	if err = json.Unmarshal(expr, &c); err != nil {
		err = fmt.Errorf("failed to unmarshal current mouse position info: %v", err)
	}
	return
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

func f2int(f float64) int {
	return int(math.Round(f))
}
