package types

import (
	"fmt"
	"math"

	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/span"
)

// TODO: we need to reflect somehow whether a buffer is file-based or not. A
// preview window is not, for example.

// A Buffer is govim's representation of the current state of a buffer in Vim
// i.e. it is versioned.
type Buffer struct {
	Num      int
	Name     string
	Contents []byte
	Version  int

	// cc is lazily set whenever position information is required
	cc *span.TokenConverter
}

// A WatchedFile is a file we are watching but that is not loaded as a buffer
// in Vim
type WatchedFile struct {
	Path     string
	Version  int
	Contents []byte
}

func (w *WatchedFile) URI() span.URI {
	return span.FileURI(w.Path)
}

// URI returns the b's Name as a span.URI, assuming it is a file.
// TODO we should panic here is this is not a file-based buffer
func (b *Buffer) URI() span.URI {
	return span.FileURI(b.Name)
}

// ToTextDocumentIdentifier converts b to a protocol.TextDocumentIdentifier
func (b *Buffer) ToTextDocumentIdentifier() protocol.TextDocumentIdentifier {
	return protocol.TextDocumentIdentifier{
		URI: string(b.URI()),
	}
}

func (b *Buffer) tokenConvertor() *span.TokenConverter {
	if b.cc == nil {
		b.cc = span.NewContentConverter(b.Name, b.Contents)
	}
	return b.cc
}

// Range represents a range within a Buffer. Create ranges using NewRange
type Range struct {
	Start Point
	End   Point
}

// Point represents a position within a Buffer
type Point struct {
	// line is Vim's line number within the buffer, i.e. 1-indexed
	line int

	// col is the Vim representation of column number, i.e.  1-based byte index
	col int

	// offset is the 0-index byte-offset
	offset int

	// is the 0-index character offset in line
	utf16Col int
}

func PointFromVim(b *Buffer, line, col int) (Point, error) {
	cc := b.tokenConvertor()
	off, err := cc.ToOffset(line, col)
	if err != nil {
		return Point{}, fmt.Errorf("failed to calculate offset within buffer %v: %v", b.Num, err)
	}
	p := span.NewPoint(line, col, off)
	utf16col, err := span.ToUTF16Column(p, b.Contents)
	if err != nil {
		return Point{}, fmt.Errorf("failed to calculate UTF16 char value: %v", err)
	}
	res := Point{
		line:     line,
		col:      col,
		offset:   off,
		utf16Col: utf16col - 1,
	}
	return res, nil
}

func PointFromPosition(b *Buffer, pos protocol.Position) (Point, error) {
	cc := b.tokenConvertor()
	sline := f2int(pos.Line) + 1
	scol := f2int(pos.Character)
	soff, err := cc.ToOffset(sline, 1)
	if err != nil {
		return Point{}, fmt.Errorf("failed to calculate offset within buffer %v: %v", b.Num, err)
	}
	p := span.NewPoint(sline, 1, soff)
	p, err = span.FromUTF16Column(p, scol+1, b.Contents)
	if err != nil {
		return Point{}, fmt.Errorf("failed to translate char colum for buffer %v: %v", b.Num, err)
	}
	res := Point{
		line:     p.Line(),
		col:      p.Column(),
		offset:   p.Offset(),
		utf16Col: scol,
	}
	return res, nil
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
func (p Point) GoplsLine() float64 {
	return float64(p.line - 1)
}

// GoplsChar is the 0-index character offset in a buffer.
func (p Point) GoplsChar() float64 {
	return float64(p.utf16Col)
}

// ToPosition converts p to a protocol.Position
func (p Point) ToPosition() protocol.Position {
	return protocol.Position{
		Line:      p.GoplsLine(),
		Character: p.GoplsChar(),
	}
}

func f2int(f float64) int {
	return int(math.Round(f))
}
