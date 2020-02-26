package main

import (
	"github.com/govim/govim/cmd/govim/internal/types"
)

type cursorPosition struct {
	BufNr int `json:"bufnr"`
	Line  int `json:"line"`
	Col   int `json:"col"`
}

const cursorPositionExpr = `{"bufnr": bufnr(""), "line": line("."), "col": col(".")}`

func (v *vimstate) cursorPos() (b *types.Buffer, p types.Point, err error) {
	var pos cursorPosition
	expr := v.ChannelExpr(cursorPositionExpr)
	v.Parse(expr, &pos)
	b, err = v.getLoadedBuffer(pos.BufNr)
	if err != nil {
		return
	}
	p, err = types.PointFromVim(b, pos.Line, pos.Col)
	return
}
