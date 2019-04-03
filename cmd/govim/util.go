package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/myitcv/govim/cmd/govim/types"
	"github.com/russross/blackfriday/v2"
)

// fetchCurrentBufferInfo is a helper function to snapshot the current buffer
// information from Vim. This helper method should only be used with methods
// responsible for updating d.buffers
func (v *vimstate) fetchCurrentBufferInfo() (*types.Buffer, error) {
	var buf struct {
		Num      int
		Name     string
		Contents string
	}
	expr := v.ChannelExpr(`{"Num": bufnr(""), "Name": expand('%:p'), "Contents": join(getline(0, "$"), "\n")}`)
	if err := json.Unmarshal(expr, &buf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal current buffer info: %v", err)
	}
	res := &types.Buffer{
		Num:      buf.Num,
		Name:     buf.Name,
		Contents: []byte(buf.Contents),
	}
	return res, nil
}

func (v *vimstate) cursorPos() (b *types.Buffer, p types.Point, err error) {
	var pos struct {
		BufNum int `json:"bufnum"`
		Line   int `json:"line"`
		Col    int `json:"col"`
	}
	expr := v.ChannelExpr(`{"bufnum": bufnr(""), "line": line("."), "col": col(".")}`)
	if err = json.Unmarshal(expr, &pos); err != nil {
		err = fmt.Errorf("failed to unmarshal current cursor position info: %v", err)
		return
	}
	b, ok := v.buffers[pos.BufNum]
	if !ok {
		err = fmt.Errorf("failed to resolve buffer %v", pos.BufNum)
		return
	}
	p, err = types.PointFromVim(b, pos.Line, pos.Col)
	return
}

func (v *vimstate) mousePos() (b *types.Buffer, p types.Point, err error) {
	var pos struct {
		BufNum int `json:"bufnum"`
		Line   int `json:"line"`
		Col    int `json:"col"`
	}
	expr := v.ChannelExpr(`{"bufnum": v:beval_bufnr, "line": v:beval_lnum, "col": v:beval_col}`)
	if err = json.Unmarshal(expr, &pos); err != nil {
		err = fmt.Errorf("failed to unmarshal current mouse position info: %v", err)
		return
	}
	b, ok := v.buffers[pos.BufNum]
	if !ok {
		err = fmt.Errorf("failed to resolve buffer %v", pos.BufNum)
		return
	}
	p, err = types.PointFromVim(b, pos.Line, pos.Col)
	return
}

type plainMarkdown struct{}

var _ blackfriday.Renderer = plainMarkdown{}

func (p plainMarkdown) RenderNode(w io.Writer, node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
	fmt.Fprint(w, string(node.Literal))
	return blackfriday.GoToNext
}

func (p plainMarkdown) RenderHeader(w io.Writer, ast *blackfriday.Node) {}
func (p plainMarkdown) RenderFooter(w io.Writer, ast *blackfriday.Node) {}
