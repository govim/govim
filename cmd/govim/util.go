package main

import (
	"encoding/json"
	"fmt"

	"github.com/govim/govim/cmd/govim/internal/types"
)

const (
	exprAutocmdCurrBufInfo = `{"Num": eval(expand('<abuf>')), "Name": fnamemodify(bufname(eval(expand('<abuf>'))),':p'), "Contents": join(getbufline(eval(expand('<abuf>')), 0, "$"), "\n")."\n", "Loaded": bufloaded(eval(expand('<abuf>')))}`
)

// currentBufferInfo is a helper function to unmarshal autocmd current
// buffer details from expr
func (v *vimstate) currentBufferInfo(expr json.RawMessage) *types.Buffer {
	var buf struct {
		Num      int
		Name     string
		Contents string
		Loaded   int
	}
	v.Parse(expr, &buf)
	return types.NewBuffer(buf.Num, buf.Name, []byte(buf.Contents), buf.Loaded == 1)
}

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
	b, ok := v.buffers[pos.BufNr]
	if !ok {
		err = fmt.Errorf("failed to resolve buffer %v", pos.BufNr)
		return
	}
	p, err = types.PointFromVim(b, pos.Line, pos.Col)
	return
}
