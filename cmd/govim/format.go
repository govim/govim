package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/myitcv/govim/cmd/govim/config"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/types"
)

func (v *vimstate) formatCurrentBuffer(args ...json.RawMessage) (err error) {
	tool := v.ParseString(v.ChannelExpr(config.GlobalFormatOnSave))
	// we are an autocmd endpoint so we need to be told the current
	// buffer number via <abuf>
	currBufNr := v.ParseInt(args[0])
	b, ok := v.buffers[currBufNr]
	if !ok {
		return fmt.Errorf("failed to resolve buffer %v", currBufNr)
	}

	var edits []protocol.TextEdit

	switch config.FormatOnSave(tool) {
	case config.FormatOnSaveNone:
		return nil
	case config.FormatOnSaveGoFmt:
		params := &protocol.DocumentFormattingParams{
			TextDocument: b.ToTextDocumentIdentifier(),
		}
		edits, err = v.server.Formatting(context.Background(), params)
		if err != nil {
			return fmt.Errorf("failed to call gopls.Formatting: %v", err)
		}
	case config.FormatOnSaveGoImports:
		params := &protocol.CodeActionParams{
			TextDocument: b.ToTextDocumentIdentifier(),
		}
		actions, err := v.server.CodeAction(context.Background(), params)
		if err != nil {
			return fmt.Errorf("failed to call gopls.CodeAction: %v", err)
		}
		switch len(actions) {
		case 0:
			return nil
		case 1:
			edits = (*actions[0].Edit.Changes)[string(b.URI())]
		default:
			return fmt.Errorf("don't know how to handle %v actions", len(actions))
		}
	default:
		return fmt.Errorf("unknown format tool specified for %v: %v", config.GlobalFormatOnSave, tool)
	}

	// see :help wundo. The use of wundo! is significant. It first deletes
	// the temp file we created, but only recreates it if there is something
	// to write.  This is inherently racey... because theorectically the file
	// might in the meantime have been created by another instance of
	// govim.... We reduce that risk using the time above
	tf, err := ioutil.TempFile("", strconv.FormatInt(time.Now().UnixNano(), 10))
	if err != nil {
		return fmt.Errorf("failed to create temp undo file")
	}

	v.ChannelExf("wundo! %v", tf.Name())
	defer func() {
		if _, err := os.Stat(tf.Name()); err != nil {
			return
		}
		v.ChannelExf("silent! rundo %v", tf.Name())
		err = os.Remove(tf.Name())
	}()

	preEventIgnore := v.ParseString(v.ChannelExpr("&eventignore"))
	v.ChannelEx("set eventignore=all")
	defer v.ChannelExf("set eventignore=%v", preEventIgnore)
	v.ToggleOnViewportChange()
	defer v.ToggleOnViewportChange()
	for ie := len(edits) - 1; ie >= 0; ie-- {
		e := edits[ie]
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
			return fmt.Errorf("saw an edit where start col != end col (range start: %v, range end: %v start: %v, end: %v). We can't currently handle this", e.Range.Start, e.Range.End, start, end)
		}

		if start.Line() != end.Line() {
			if e.NewText != "" {
				return fmt.Errorf("saw an edit where start line != end line with replacement text %q; We can't currently handle this", e.NewText)
			}
			// This is a delete of line
			if res := v.ParseInt(v.ChannelCall("deletebufline", b.Num, start.Line(), end.Line()-1)); res != 0 {
				return fmt.Errorf("deletebufline(%v, %v, %v) failed", b.Num, start.Line(), end.Line()-1)
			}
		} else {
			// do we have anything to do?
			if e.NewText == "" {
				continue
			}
			// we are within the same line so strip the newline
			if e.NewText[len(e.NewText)-1] == '\n' {
				e.NewText = e.NewText[:len(e.NewText)-1]
			}
			repl := strings.Split(e.NewText, "\n")
			v.ChannelCall("append", start.Line()-1, repl)
		}
	}
	return nil
}
