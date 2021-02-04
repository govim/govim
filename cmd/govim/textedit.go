package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/types"
)

type textEdit struct {
	buffer int
	call   string
	start  int
	end    int
	lines  []string
}

func (v *vimstate) applyProtocolTextEdits(b *types.Buffer, edits []protocol.TextEdit) error {

	// prepare the changes to make in Vim
	blines := bytes.Split(b.Contents()[:len(b.Contents())-1], []byte("\n"))
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
		// Skip empty edits
		if start == end && e.NewText == "" {
			continue
		}
		// special case deleting of complete lines
		if start.Col() == 1 && end.Col() == 1 && e.NewText == "" {
			delstart := min(start.Line(), len(blines))
			delend := min(end.Line()-1, len(blines))
			changes = append(changes, textEdit{
				call:   "deletebufline",
				buffer: b.Num,
				start:  delstart,
				end:    delend,
			})
			blines = append(blines[:delstart-1], blines[delend:]...)
			continue
		}
		newLines := strings.Split(e.NewText, "\n")
		appendAdjust := 1
		if start.Line()-1 < len(blines) {
			appendAdjust = 0
			startLine := blines[start.Line()-1]
			newLines[0] = string(startLine[:start.Col()-1]) + newLines[0]
			if end.Line()-1 < len(blines) {
				endLine := blines[end.Line()-1]
				newLines[len(newLines)-1] += string(endLine[end.Col()-1:])
			}
			// we only need to update the start line because we can't have
			// overlapping edits
			blines[start.Line()-1] = []byte(newLines[0])
			changes = append(changes, textEdit{
				call:   "setbufline",
				buffer: b.Num,
				start:  start.Line(),
				lines:  []string{string(blines[start.Line()-1])},
			})
		} else {
			blines = append(blines, []byte(newLines[0]))
		}
		if start.Line() != end.Line() {
			// We can't delete beyond the end of the buffer. So the start end end here are
			// both min() reduced
			delstart := min(start.Line()+1, len(blines))
			delend := min(end.Line(), len(blines))
			changes = append(changes, textEdit{
				call:   "deletebufline",
				buffer: b.Num,
				start:  delstart,
				end:    delend,
			})
			blines = blines[:delstart-1]
		}
		if len(newLines) > 1 {
			changes = append(changes, textEdit{
				call:   "appendbufline",
				buffer: b.Num,
				start:  start.Line() - appendAdjust,
				lines:  newLines[1-appendAdjust : len(newLines)-appendAdjust],
			})
		}
	}

	// see :help wundo. The use of wundo! is significant. It first deletes
	// the temp file we created, but only recreates it if there is something
	// to write.  This is inherently racey... because theorectically the file
	// might in the meantime have been created by another instance of
	// govim.... We reduce that risk using the time above
	tf, err := ioutil.TempFile("", strconv.FormatInt(time.Now().UnixNano(), 10))
	if err != nil {
		return fmt.Errorf("failed to create temp undo file: %v", err)
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
	v.ChannelCall("listener_remove", b.Listener)
	defer func() {
		b.Listener = v.ParseInt(v.ChannelCall("listener_add", v.Prefix()+string(config.FunctionEnrichDelta), b.Num))
	}()
	v.BatchStart()
	for _, e := range changes {
		switch e.call {
		case "setbufline":
			v.BatchAssertChannelCall(AssertIsZero(), e.call, b.Num, e.start, e.lines[0])
		case "deletebufline":
			v.BatchAssertChannelCall(AssertIsZero(), e.call, b.Num, e.start, e.end)
		case "appendbufline":
			v.BatchAssertChannelCall(AssertIsZero(), e.call, b.Num, e.start, e.lines)
		default:
			panic(fmt.Errorf("unknown change type: %v", e.call))
		}
	}
	v.BatchAssertChannelCall(AssertIsZero(), "listener_flush", b.Num)
	newContentsRes := v.BatchChannelExprf(`join(getbufline(%v, 0, "$"), "\n")."\n"`, b.Num)
	v.MustBatchEnd()

	var newContents string
	v.Parse(newContentsRes(), &newContents)
	b.SetContents([]byte(newContents))
	v.triggerBufferASTUpdate(b)
	b.Version++
	params := &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: b.ToTextDocumentIdentifier(),
			Version:                b.Version,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Text: newContents,
			},
		},
	}

	return v.server.DidChange(context.Background(), params)
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}
