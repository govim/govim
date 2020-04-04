package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
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

// cursorPos returns the current cursor position cp. If the current buffer
// (i.e. the buffer behind the window in which the cursor is currently found)
// is being tracked by govim then cp.Point will be set, otherwise it will be
// nil.
func (v *vimstate) cursorPos() (types.CursorPosition, error) {
	return v.parseCursorPos(v.ChannelExpr("s:cursorPos()"))
}

// bufCursorPos returns the current buffer b and current cursor position cp if
// the current buffer is being tracked by govim. Otherwise it returns an error.
func (v *vimstate) bufCursorPos() (b *types.Buffer, cp types.CursorPosition, err error) {
	cp, err = v.parseCursorPos(v.ChannelExpr("s:cursorPos()"))
	if err != nil {
		return
	}
	if cp.Point == nil {
		err = fmt.Errorf("cursor position in buffer %v not tracked by govim", cp.BufNr)
		return
	}
	b = cp.Point.Buffer()
	return
}

// parseCursorPos expects a json.RawMessage of the structure returned by s:cursorPos
// from plugin/govim.vim.
func (v *vimstate) parseCursorPos(r json.RawMessage) (types.CursorPosition, error) {
	var cp types.CursorPosition
	var pos struct {
		BufNr     int `json:"bufnr"`
		Line      int `json:"line"`
		Col       int `json:"col"`
		WinNr     int `json:"winnr"`
		WinID     int `json:"winid"`
		ScreenPos struct {
			Row     int `json:"row"`
			Col     int `json:"col"`
			EndCol  int `json:"endcol"`
			CursCol int `json:"curscol"`
		} `json:"screenpos"`
	}
	v.Parse(r, &pos)
	var p *types.Point
	b, ok := v.buffers[pos.BufNr]
	if ok {
		pt, err := types.PointFromVim(b, pos.Line, pos.Col)
		if err != nil {
			return cp, fmt.Errorf("failed to create Point: %v", err)
		}
		p = &pt
	}
	cp = types.CursorPosition{
		Point:         p,
		BufNr:         pos.BufNr,
		WinNr:         pos.WinNr,
		WinID:         pos.WinID,
		ScreenRow:     pos.ScreenPos.Row,
		ScreenCol:     pos.ScreenPos.Col,
		ScreenEndCol:  pos.ScreenPos.EndCol,
		ScreenCursCol: pos.ScreenPos.CursCol,
	}
	return cp, nil
}

func (v *vimstate) locationToQuickfix(loc protocol.Location, rel bool) (qf quickfixEntry, err error) {
	var buf *types.Buffer
	for _, b := range v.buffers {
		if b.Loaded && b.URI() == span.URI(loc.URI) {
			buf = b
		}
	}
	fn := span.URI(loc.URI).Filename()
	if buf == nil {
		byts, err := ioutil.ReadFile(fn)
		if err != nil {
			return qf, fmt.Errorf("failed to read contents of %v: %v", fn, err)
		}
		// create a temp buffer
		buf = types.NewBuffer(-1, fn, byts, false)
	}
	// make fn relative for reporting purposes
	if rel {
		fn, err = filepath.Rel(v.workingDirectory, fn)
		if err != nil {
			return qf, fmt.Errorf("failed to call filepath.Rel(%q, %q): %v", v.workingDirectory, fn, err)
		}
	}
	p, err := types.PointFromPosition(buf, loc.Range.Start)
	if err != nil {
		return qf, fmt.Errorf("failed to resolve position: %v", err)
	}
	line, err := buf.Line(p.Line())
	if err != nil {
		return qf, fmt.Errorf("location invalid in buffer: %v", err)
	}
	qf = quickfixEntry{
		Filename: fn,
		Lnum:     p.Line(),
		Col:      p.Col(),
		Text:     line,
	}
	return qf, nil
}

// populateQuickfix populates and opens a quickfix window with a sorted
// slice of locations. If shift is true the first element of the slice
// will be skipped.
func (v *vimstate) populateQuickfix(locs []protocol.Location, shift bool) {
	var qfs []quickfixEntry
	for _, l := range locs {
		qf, err := v.locationToQuickfix(l, true)
		if err != nil {
			// TODO: come up with a better strategy for alerting the user to the
			// fact that the conversation to quickfix entries failed. Should be rare
			// but when it does happen, we need to be noisy.
			v.Logf("failed to convert locations to quickfix entries: %v", err)
			return
		}
		qfs = append(qfs, qf)
	}

	var toSort []quickfixEntry

	if shift {
		toSort = qfs[1:]
	} else {
		toSort = qfs
	}

	sort.Slice(toSort, func(i, j int) bool {
		lhs, rhs := toSort[i], toSort[j]
		cmp := strings.Compare(lhs.Filename, rhs.Filename)
		if cmp != 0 {
			if lhs.Filename == qfs[0].Filename {
				return true
			} else if rhs.Filename == qfs[0].Filename {
				return false
			}
		}
		if cmp == 0 {
			cmp = lhs.Lnum - rhs.Lnum
		}
		if cmp == 0 {
			cmp = lhs.Col - rhs.Col
		}
		return cmp < 0
	})

	v.ChannelCall("setqflist", qfs, "r")
	v.ChannelEx("copen")
}

func (v *vimstate) parentCommand(args ...json.RawMessage) (interface{}, error) {
	return v.parentCallArgs, nil
}
