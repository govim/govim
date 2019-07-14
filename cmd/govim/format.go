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

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/config"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/types"
)

func (v *vimstate) formatCurrentBuffer(args ...json.RawMessage) (err error) {
	// we are an autocmd endpoint so we need to be told the current
	// buffer number via <abuf>
	currBufNr := v.ParseInt(args[0])
	b, ok := v.buffers[currBufNr]
	if !ok {
		return fmt.Errorf("failed to resolve buffer %v", currBufNr)
	}
	tool := v.config.FormatOnSave
	// TODO we should move this validation elsewhere...
	switch tool {
	case config.FormatOnSaveNone:
		return nil
	case config.FormatOnSaveGoFmt, config.FormatOnSaveGoImports:
	default:
		return fmt.Errorf("unknown format tool specified: %v", tool)
	}
	return v.formatBufferRange(b, tool, govim.CommandFlags{})
}

func (v *vimstate) gofmtCurrentBufferRange(flags govim.CommandFlags, args ...string) error {
	return v.formatCurrentBufferRange(config.FormatOnSaveGoFmt, flags, args...)
}

func (v *vimstate) goimportsCurrentBufferRange(flags govim.CommandFlags, args ...string) error {
	return v.formatCurrentBufferRange(config.FormatOnSaveGoImports, flags, args...)
}

func (v *vimstate) formatCurrentBufferRange(mode config.FormatOnSave, flags govim.CommandFlags, args ...string) error {
	vp := v.Viewport()
	b, ok := v.buffers[vp.Current.BufNr]
	if !ok {
		return fmt.Errorf("failed to resolve current buffer %v", vp.Current.BufNr)
	}
	return v.formatBufferRange(b, mode, flags)
}

func (v *vimstate) formatBufferRange(b *types.Buffer, mode config.FormatOnSave, flags govim.CommandFlags, args ...string) error {
	var err error
	var edits []protocol.TextEdit

	var ran *protocol.Range
	if flags.Range != nil {
		start, err := types.PointFromVim(b, *flags.Line1, 1)
		if err != nil {
			return fmt.Errorf("failed to convert start of range (%v, 1) to Point: %v", *flags.Line1, err)
		}
		end, err := types.PointFromVim(b, *flags.Line2+1, 1)
		if err != nil {
			return fmt.Errorf("failed to convert end of range (%v, 1) to Point: %v", *flags.Line2, err)
		}
		ran = &protocol.Range{
			Start: start.ToPosition(),
			End:   end.ToPosition(),
		}
	}

	switch mode {
	case config.FormatOnSaveGoFmt:
		if flags.Range != nil {
			params := &protocol.DocumentRangeFormattingParams{
				TextDocument: b.ToTextDocumentIdentifier(),
				Range:        *ran,
			}
			edits, err = v.server.RangeFormatting(context.Background(), params)
			if err != nil {
				v.Logf("gopls.RangeFormatting returned an error; nothing to do")
				return nil
			}
		} else {
			params := &protocol.DocumentFormattingParams{
				TextDocument: b.ToTextDocumentIdentifier(),
			}
			edits, err = v.server.Formatting(context.Background(), params)
			if err != nil {
				v.Logf("gopls.Formatting returned an error; nothing to do")
				return nil
			}
		}
	case config.FormatOnSaveGoImports:
		params := &protocol.CodeActionParams{
			TextDocument: b.ToTextDocumentIdentifier(),
		}
		if flags.Range != nil {
			params.Range = *ran
		}
		actions, err := v.server.CodeAction(context.Background(), params)
		if err != nil {
			v.Logf("gopls.CodeAction returned an error; nothing to do")
			return nil
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
		return fmt.Errorf("unknown format mode specified: %v", mode)
	}

	if len(edits) == 0 {
		return nil
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

	// prepare the changes to make in Vim
	var changes []textEdit
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
			changes = append(changes, textEdit{
				call:   "deletebufline",
				buffer: b.Num,
				start:  start.Line(),
				end:    end.Line() - 1,
			})
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
			changes = append(changes, textEdit{
				call:   "appendbufline",
				buffer: b.Num,
				start:  start.Line() - 1,
				lines:  repl,
			})
		}
	}

	preEventIgnore := v.ParseString(v.ChannelExpr("&eventignore"))
	v.ChannelEx("set eventignore=all")
	defer v.ChannelExf("set eventignore=%v", preEventIgnore)
	v.BatchStart()
	for _, e := range changes {
		switch e.call {
		case "deletebufline":
			v.BatchAssertChannelCall(AssertIsZero, "deletebufline", b.Num, e.start, e.end)
		case "appendbufline":
			v.BatchAssertChannelCall(AssertIsZero, "appendbufline", b.Num, e.start, e.lines)
		}
	}
	if v.doIncrementalSync() {
		v.BatchAssertChannelCall(AssertIsZero, "listener_flush", b.Num)
	}
	v.BatchEnd()
	return nil
}

type textEdit struct {
	buffer int
	call   string
	start  int
	end    int
	lines  []string
}
