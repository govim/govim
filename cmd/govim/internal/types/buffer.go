package types

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

// A Buffer is govim's representation of the current state of a buffer in Vim
// i.e. it is versioned.
//
// TODO: we need to reflect somehow whether a buffer is file-based or not. A
// preview window is not, for example.
type Buffer struct {
	Num      int
	Name     string
	contents []byte
	Version  int32

	// Listener is the ID of the listener for the buffer. Listeners number from
	// 1 so the zero value indicates this buffer does not have a listener.
	Listener int

	// Loaded reflects vim's "loaded" buffer state. See :help bufloaded() for details.
	Loaded bool

	// AST is the parsed result of the Buffer. Buffer events (i.e. changes to
	// the buffer contents) trigger an asynchronous re-parse of the buffer.
	// These events are triggered from the *vimstate thread. Any subsequent
	// (subsequent to the buffer event) attempt to use the current AST (which by
	// definition must be on the *vimstate thread) must wait for the
	// asnychronous parse to complete. This is achieved by the ASTWait channel
	// which is closed when parsing completes. Access to AST and Fset must
	// therefore be guarded by a receive on ASTWait.

	// Fset is the fileset used in parsing the buffer contents. Access to Fset
	// must be guarded by a receive on ASTWait.
	Fset *token.FileSet

	// AST is the parsed result of the Buffer. Access to Fset must be guarded by
	// a receive on ASTWait.
	AST *ast.File

	// ASTWait is used to sychronise access to AST and Fset.
	ASTWait chan bool

	// cc is lazily set whenever position information is required
	cc *span.TokenConverter
}

func NewBuffer(num int, name string, contents []byte, loaded bool) *Buffer {
	return &Buffer{
		Num:      num,
		Name:     name,
		contents: contents,
		Loaded:   loaded,
	}
}

// Contents returns a Buffer's contents. These contents must not be
// mutated. To update a Buffer's contents, call SetContents
func (b *Buffer) Contents() []byte {
	return b.contents
}

// SetContents updates a Buffer's contents to byts
func (b *Buffer) SetContents(byts []byte) {
	b.contents = byts
	b.cc = nil
}

// URI returns the b's Name as a span.URI, assuming it is a file.
//
// TODO: we should panic here is this is not a file-based buffer
func (b *Buffer) URI() span.URI {
	return span.URIFromPath(b.Name)
}

// ToTextDocumentIdentifier converts b to a protocol.TextDocumentIdentifier
func (b *Buffer) ToTextDocumentIdentifier() protocol.TextDocumentIdentifier {
	return protocol.TextDocumentIdentifier{
		URI: protocol.DocumentURI(b.URI()),
	}
}

func (b *Buffer) tokenConvertor() *span.TokenConverter {
	if b.cc == nil {
		b.cc = span.NewContentConverter(b.Name, b.contents)
	}
	return b.cc
}

// Line returns the 1-indexed line contents of b
func (b *Buffer) Line(n int) (string, error) {
	// TODO: this is inefficient because we are splitting the contents of
	// the buffer again... even thought this may already have been done
	// in the content converter, b.cc
	lines := bytes.Split(b.Contents(), []byte("\n"))
	if n >= len(lines) {
		return "", fmt.Errorf("line %v is beyond the end of the buffer (no. of lines %v)", n, len(lines))
	}
	return string(lines[n-1]), nil
}
