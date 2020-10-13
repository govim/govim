package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/config"
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

func (v *vimstate) rangeFromFlags(b *types.Buffer, flags govim.CommandFlags) (start, end types.Point, err error) {
	switch *flags.Range {
	case 2:
		// we have a range
		var pos struct {
			Mode  string `json:"mode"`
			Start []int  `json:"start"` // [bufnr, line, col, off]
			End   []int  `json:"end"`   // [bufnr, line, col, off]
		}
		v.Parse(v.ChannelExpr(`{"buffnr": bufnr(""), "mode": visualmode(), "start": getpos("'<"), "end": getpos("'>")}`), &pos)

		if pos.Mode == "\x16" { // <CTRL-V>, block-wise
			return start, end, fmt.Errorf("cannot use %v in visual block mode", config.CommandStringFn)
		}

		if pos.Mode == "V" || pos.Mode == "" {
			// There are a couple of different ways to execute range command,
			// for example :%GOVIMFooBar that doesn't set any markers (<','>).
			// Use Line1/Line2 over pos.Start/pos.End to support them.
			start, err = types.PointFromVim(b, *flags.Line1, 1)
			if err != nil {
				return start, end, fmt.Errorf("failed to get start position of range: %v", err)
			}
			// Since the end col will be "a large value" we need to evaluate
			// the real col by getting the offset for the first column on the
			// "next line" and subtract 1 (the newline).
			var nl types.Point
			nl, err = types.PointFromVim(b, *flags.Line2+1, 1)
			if err != nil {
				return start, end, fmt.Errorf("failed to get point from line after end line: %v", err)
			}
			end, err = types.PointFromOffset(b, nl.Offset()-1)
			if err != nil {
				return start, end, fmt.Errorf("failed to get end position of range: %v", err)
			}
		} else if pos.Mode == "v" {
			start, err = types.PointFromVim(b, pos.Start[1], pos.Start[2])
			if err != nil {
				return start, end, fmt.Errorf("failed to get start position of range: %v", err)
			}
			end, err = types.PointFromVim(b, pos.End[1], pos.End[2])
			if err != nil {
				return start, end, fmt.Errorf("failed to get end position of range: %v", err)
			}
			// we need to move past the end of the selection
			rem := b.Contents()[end.Offset():]
			if len(rem) > 0 {
				_, adj := utf8.DecodeRune(rem)
				end, err = types.PointFromVim(b, pos.End[1], pos.End[2]+adj)
				if err != nil {
					return start, end, fmt.Errorf("failed to get adjusted end position: %v", err)
				}
			}
		}
	case 0:
		// current line
		start, err = types.PointFromVim(b, *flags.Line1, 1)
		if err != nil {
			return start, end, fmt.Errorf("failed to derive start position from cursor position on line %v: %v", *flags.Line1, err)
		}
		lines := bytes.Split(b.Contents(), []byte("\n"))
		end, err = types.PointFromVim(b, *flags.Line1, len(lines[*flags.Line1-1])+1)
		if err != nil {
			return start, end, fmt.Errorf("failed to derive end position from cursor position on line %v: %v", *flags.Line1, err)
		}
	default:
		return start, end, fmt.Errorf("unknown range indicator %v", *flags.Range)
	}
	return start, end, nil
}
