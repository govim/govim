package main

import (
	"github.com/govim/govim/cmd/govim/internal/types"
)

func (v *vimstate) cursorPos() (b *types.Buffer, p types.Point, err error) {
	var pos struct {
		BufNum int `json:"bufnum"`
		Line   int `json:"line"`
		Col    int `json:"col"`
	}
	expr := v.ChannelExpr(`{"bufnum": bufnr(""), "line": line("."), "col": col(".")}`)
	v.Parse(expr, &pos)
	b, err = v.getLoadedBuffer(pos.BufNum)
	if err != nil {
		return
	}
	p, err = types.PointFromVim(b, pos.Line, pos.Col)
	return
}
