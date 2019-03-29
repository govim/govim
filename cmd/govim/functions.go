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
	"github.com/myitcv/govim/cmd/govim/types"
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
	b, pos, err := d.mousePos()
	if err != nil {
		return nil, fmt.Errorf("failed to determine mouse position: %v", err)
	}
	go func() {
		params := &protocol.TextDocumentPositionParams{
			TextDocument: b.ToTextDocumentIdentifier(),
			Position:     pos.ToPosition(),
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

func (d *driver) handleBufferEvent(b *types.Buffer) error {
	d.buffers[b.Num] = b

	if b.Version == 0 {
		params := &protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{
				URI:     string(b.URI()),
				Version: float64(b.Version),
				Text:    string(b.Contents),
			},
		}
		return d.server.DidOpen(context.Background(), params)
	}

	params := &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: b.ToTextDocumentIdentifier(),
			Version:                float64(b.Version),
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
			TextDocument: b.ToTextDocumentIdentifier(),
		}
		d.Logf("Calling gopls.Formatting: %v", pretty.Sprint(params))
		edits, err = d.server.Formatting(context.Background(), params)
		if err != nil {
			return fmt.Errorf("failed to call gopls.Formatting: %v", err)
		}
	case config.FormatOnSaveGoImports:
		params := &protocol.CodeActionParams{
			TextDocument: b.ToTextDocumentIdentifier(),
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
		edits = (*actions[0].Edit.Changes)[string(b.URI())]
	default:
		return fmt.Errorf("unknown format tool specified for %v: %v", config.GlobalFormatOnSave, tool)
	}

	preEventIgnore := d.ParseString(d.ChannelExpr("&eventignore"))
	d.ChannelEx("set eventignore=all")
	defer d.ChannelExf("set eventignore=%v", preEventIgnore)
	d.ToggleOnViewportChange()
	defer d.ToggleOnViewportChange()
	for ie := len(edits) - 1; ie >= 0; ie-- {
		e := edits[ie]
		d.Logf("==================")
		d.Logf("%v", pretty.Sprint(e))
		start, err := types.PointFromPosition(b, e.Range.Start)
		if err != nil {
			return fmt.Errorf("failed to derive start point from position: %v", err)
		}
		end, err := types.PointFromPosition(b, e.Range.End)
		if err != nil {
			return fmt.Errorf("failed to derive end point from position: %v", err)
		}

		if start.Col() != 1 || end.Col() != 1 {
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
