package types

import (
	"fmt"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

// Point represents a position within a Buffer
type Point struct {
	// buffer is the buffer corresponding to the Point
	buffer *Buffer

	// line is Vim's line number within the buffer, i.e. 1-indexed
	line int

	// col is the Vim representation of column number, i.e.  1-based byte index
	col int

	// offset is the 0-index byte-offset
	offset int

	// is the 0-index character offset in line
	utf16Col int
}

func PointFromOffset(b *Buffer, offset int) (Point, error) {
	tf := b.tokFile()
	line, col, err := span.ToPosition(tf, offset)
	if err != nil {
		return Point{}, fmt.Errorf("failed to calculate position within buffer %v: %v", b.Num, err)
	}
	p := span.NewPoint(line, col, offset)
	utf16col, err := span.ToUTF16Column(p, b.contents)
	if err != nil {
		return Point{}, fmt.Errorf("failed to calculate UTF16 char value: %v", err)
	}
	res := Point{
		buffer:   b,
		line:     line,
		col:      col,
		offset:   offset,
		utf16Col: utf16col - 1,
	}
	return res, nil
}

func PointFromVim(b *Buffer, line, col int) (Point, error) {
	tf := b.tokFile()
	off, err := span.ToOffset(tf, line, col)
	if err != nil {
		return Point{}, fmt.Errorf("failed to calculate offset within buffer %v: %v", b.Num, err)
	}
	p := span.NewPoint(int(line), int(col), off)
	utf16col, err := span.ToUTF16Column(p, b.contents)
	if err != nil {
		return Point{}, fmt.Errorf("failed to calculate UTF16 char value: %v", err)
	}
	res := Point{
		buffer:   b,
		line:     line,
		col:      col,
		offset:   off,
		utf16Col: utf16col - 1,
	}
	return res, nil
}

func PointFromPosition(b *Buffer, pos protocol.Position) (Point, error) {
	tf := b.tokFile()
	sline := pos.Line + 1
	scol := pos.Character
	soff, err := span.ToOffset(tf, int(sline), 1)
	if err != nil {
		return Point{}, fmt.Errorf("failed to calculate offset within buffer %v: %v", b.Num, err)
	}
	p := span.NewPoint(int(sline), 1, soff)
	p, err = span.FromUTF16Column(p, int(scol)+1, b.contents)
	if err != nil {
		return Point{}, fmt.Errorf("failed to translate char colum for buffer %v: %v", b.Num, err)
	}
	res := Point{
		buffer:   b,
		line:     p.Line(),
		col:      p.Column(),
		offset:   p.Offset(),
		utf16Col: int(scol),
	}
	return res, nil
}

func VisualPointFromPosition(b *Buffer, pos protocol.Position) (Point, error) {
	p, err := PointFromPosition(b, pos)
	if err != nil {
		return p, err
	}
	c := b.Contents()
	l := len(c)
	if p.Offset() == l && l > 0 && c[l-1] == '\n' {
		tf := b.tokFile()
		var newLine, newCol int
		newLine, newCol, err = span.ToPosition(tf, l-1)
		if err != nil {
			return p, err
		}
		p, err = PointFromVim(b, newLine, newCol)
	}
	return p, err
}

// Buffer is the buffer corresponding to the Point
func (p Point) Buffer() *Buffer {
	return p.buffer
}

// Line refers to the 1-indexed line in the buffer. This is how Vim refers to
// line numbers.
func (p Point) Line() int {
	return p.line
}

// Col refers to the byte index (1-based) in Line() in the buffer. This is
// often referred to as the column number, but is definitely not the visual
// column as seen on screen. This is how Vim refers to column positions.
func (p Point) Col() int {
	return p.col
}

// Offset represents the byte offset (0-indexed) of p within p.Buffer()
func (p Point) Offset() int {
	return p.offset
}

// GoplsLine is the 0-index line in the buffer, returned as a float64 value. This
// is how gopls refers to lines.
func (p Point) GoplsLine() uint32 {
	return uint32(p.line) - 1
}

// GoplsChar is the 0-index character offset in a buffer.
func (p Point) GoplsChar() uint32 {
	return uint32(p.utf16Col)
}

// ToPosition converts p to a protocol.Position
func (p Point) ToPosition() protocol.Position {
	return protocol.Position{
		Line:      p.GoplsLine(),
		Character: p.GoplsChar(),
	}
}

// IsWithin returns true if a point is within the given range
func (p Point) IsWithin(r Range) bool {
	return r.Start.Offset() <= p.Offset() &&
		p.Offset() <= r.End.Offset()
}
