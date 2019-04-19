package main

import (
	"encoding/json"
	"fmt"

	"github.com/myitcv/govim/cmd/govim/types"
)

const (
	exprAutocmdCurrBufInfo = `{"Num": eval(expand('<abuf>')), "Name": expand('<afile>:p'), "Contents": join(getbufline(eval(expand('<abuf>')), 0, "$"), "\n")}`
)

// currentBufferInfo is a helper function to unmarshal autocmd current
// buffer details from expr
func (v *vimstate) currentBufferInfo(expr json.RawMessage) (*types.Buffer, error) {
	var buf struct {
		Num      int
		Name     string
		Contents string
	}
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
