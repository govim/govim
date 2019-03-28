package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kr/pretty"
	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/span"
	"github.com/russross/blackfriday/v2"
)

func (d *driver) hello(args ...json.RawMessage) (interface{}, error) {
	return "Hello from function", nil
}

func (d *driver) helloComm(flags govim.CommandFlags, args ...string) error {
	d.ChannelEx(`echom "Hello from command"`)
	return nil
}

func (d *driver) balloonExpr(args ...json.RawMessage) (interface{}, error) {
	pos, err := d.mousePos()
	if err != nil {
		return nil, fmt.Errorf("failed to get current position: %v", err)
	}
	b, ok := d.buffers[pos.BufNum]
	if !ok {
		return nil, fmt.Errorf("failed to resolve buffer from buffer number %v", pos.BufNum)
	}
	cc := span.NewContentConverter(b.Name, b.Contents)
	off, err := cc.ToOffset(pos.Line, pos.Col)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate mouse position offset: %v", err)
	}
	p := span.NewPoint(pos.Line, pos.Col, off)
	col, err := span.ToUTF16Column(p, b.Contents)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate UTF col number of mouse position: %v", err)
	}
	go func() {
		params := &protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: string(span.FileURI(b.Name)),
			},
			Position: protocol.Position{
				Line:      float64(pos.Line - 1),
				Character: float64(col - 1),
			},
		}
		d.Logf("calling Hover: %v", pretty.Sprint(params))
		hovRes, err := d.server.Hover(context.Background(), params)
		if err != nil {
			d.ChannelCall("balloon_show", fmt.Sprintf("failed to get hover details: %v", err))
		} else {
			d.Logf("got Hover results: %q", hovRes.Contents.Value)
			md := []byte(hovRes.Contents.Value)
			plain := string(blackfriday.Run(md, blackfriday.WithRenderer(plainMarkdown{})))
			plain = strings.TrimSpace(plain)
			d.ChannelCall("balloon_show", strings.Split(plain, "\n"))
		}

	}()
	return "", nil
}

func (d *driver) bufReadPost() error {
	b, err := d.currentBuffer()
	if err != nil {
		return err
	}
	if _, ok := d.buffers[b.Num]; ok {
		return fmt.Errorf("have already seen buffer %v (%v) - this should be impossible", b.Num, b.Name)
	}
	b.Version = 0
	d.buffers[b.Num] = b
	params := &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     string(span.FileURI(b.Name)),
			Version: float64(b.Version),
			Text:    string(b.Contents),
		},
	}
	return d.server.DidOpen(context.Background(), params)
}

func (d *driver) bufTextChanged() error {
	b, err := d.currentBuffer()
	if err != nil {
		return err
	}
	cb, ok := d.buffers[b.Num]
	if !ok {
		return fmt.Errorf("have not seen buffer %v (%v) - this should be impossible", b.Num, b.Name)
	}
	b.Version = cb.Version + 1
	d.buffers[b.Num] = b
	params := &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: string(span.FileURI(b.Name)),
			},
			Version: float64(b.Version),
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Text: string(b.Contents),
			},
		},
	}
	return d.server.DidChange(context.Background(), params)
}

func (d *driver) bad(args ...json.RawMessage) (interface{}, error) {
	return nil, fmt.Errorf("this is a bad function")
}
