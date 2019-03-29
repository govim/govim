package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kr/pretty"
	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/config"
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
	b, err := d.fetchCurrentBufferInfo()
	if err != nil {
		return err
	}
	if cb, ok := d.buffers[b.Num]; ok {
		// reload of buffer, e.g. e!
		b.Version = cb.Version + 1
	} else {
		b.Version = 0
	}
	return d.handleBufferEvent(b)
}

func (d *driver) bufTextChanged() error {
	b, err := d.fetchCurrentBufferInfo()
	if err != nil {
		return err
	}
	cb, ok := d.buffers[b.Num]
	if !ok {
		return fmt.Errorf("have not seen buffer %v (%v) - this should be impossible", b.Num, b.Name)
	}
	b.Version = cb.Version + 1
	return d.handleBufferEvent(b)
}

func (d *driver) handleBufferEvent(b Buffer) error {
	d.buffers[b.Num] = b

	if b.Version == 0 {
		params := &protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{
				URI:     string(span.FileURI(b.Name)),
				Version: float64(b.Version),
				Text:    string(b.Contents),
			},
		}
		return d.server.DidOpen(context.Background(), params)
	}

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

func (d *driver) formatCurrentBuffer() error {
	var err error
	tool := d.ParseString(d.ChannelExpr(config.GlobalFormatOnSave))
	v := d.Viewport()
	b := d.buffers[v.Current.BufNr]

	var edits []protocol.TextEdit

	switch config.FormatOnSave(tool) {
	case config.FormatOnSaveGoFmt:
		params := &protocol.DocumentFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: string(span.FileURI(b.Name)),
			},
		}
		d.Logf("Calling gopls.Formatting: %v", pretty.Sprint(params))
		edits, err = d.server.Formatting(context.Background(), params)
		if err != nil {
			return fmt.Errorf("failed to call gopls.Formatting: %v", err)
		}
	case config.FormatOnSaveGoImports:
		uri := string(span.FileURI(b.Name))
		params := &protocol.CodeActionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: uri,
			},
		}
		d.Logf("Calling gopls.CodeAction: %v", pretty.Sprint(params))
		actions, err := d.server.CodeAction(context.Background(), params)
		if err != nil {
			return fmt.Errorf("failed to call gopls.CodeAction: %v", err)
		}
		want := 1
		if got := len(actions); want != got {
			return fmt.Errorf("got %v actions; expected %v", got, want)
		}
		edits = (*actions[0].Edit.Changes)[uri]
	default:
		return fmt.Errorf("unknown format tool specified for %v: %v", config.GlobalFormatOnSave, tool)
	}

	cc := span.NewContentConverter(b.Name, b.Contents)
	preEventIgnore := d.ParseString(d.ChannelExpr("&eventignore"))
	d.ChannelEx("set eventignore=all")
	defer d.ChannelExf("set eventignore=%v", preEventIgnore)
	d.ToggleOnViewportChange()
	defer d.ToggleOnViewportChange()
	for ie := len(edits) - 1; ie >= 0; ie-- {
		e := edits[ie]
		d.Logf("==================")
		d.Logf("%v", pretty.Sprint(e))
		sline := f2int(e.Range.Start.Line)
		schar := f2int(e.Range.Start.Character)
		eline := f2int(e.Range.End.Line)
		echar := f2int(e.Range.End.Character)
		soff, err := cc.ToOffset(sline+1, 0)
		if err != nil {
			return fmt.Errorf("failed to calculate start position offset for %v: %v", e.Range.Start, err)
		}
		eoff, err := cc.ToOffset(eline+1, 0)
		if err != nil {
			return fmt.Errorf("failed to calculate start position offset for %v: %v", e.Range.Start, err)
		}
		start := span.NewPoint(sline+1, 0, soff)
		end := span.NewPoint(eline+1, 0, eoff)

		if e.Range.Start.Character > 0 {
			start, err = span.FromUTF16Column(start, schar, b.Contents)
			if err != nil {
				return fmt.Errorf("failed to adjust start colum for %v: %v", e.Range.Start, err)
			}
		}
		if e.Range.End.Character > 0 {
			end, err = span.FromUTF16Column(end, echar, b.Contents)
			if err != nil {
				return fmt.Errorf("failed to adjust end colum for %v: %v", e.Range.End, err)
			}
		}

		if start.Column() != 1 || end.Column() != 1 {
			// Whether this is a delete or not, we will implement support for this later
			return fmt.Errorf("saw an edit where start col != end col (edit: %v). We can't currently handle this", e)
		}

		if start.Line() != end.Line() {
			if e.NewText != "" {
				return fmt.Errorf("saw an edit where start line != end line with replacement text %q; We can't currently handle this", e.NewText)
			}
			// This is a delete of line
			if res := d.ParseInt(d.ChannelCall("deletebufline", b.Num, start.Line(), end.Line()-1)); res != 0 {
				return fmt.Errorf("deletebufline(%v, %v, %v) failed", b.Num, start.Line(), end.Line()-1)
			}
		} else {
			// we are within the same line so strip the newline
			if e.NewText != "" && e.NewText[len(e.NewText)-1] == '\n' {
				e.NewText = e.NewText[:len(e.NewText)-1]
			}
			repl := strings.Split(e.NewText, "\n")
			d.ChannelCall("append", start.Line()-1, repl)
		}
	}
	return d.bufTextChanged()
}
