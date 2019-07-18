package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/myitcv/govim/cmd/govim/config"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/types"
)

func (v *vimstate) bufReadPost(args ...json.RawMessage) error {
	nb := v.currentBufferInfo(args[0])
	if cb, ok := v.buffers[nb.Num]; ok {
		// reload of buffer, e.v. e!
		cb.SetContents(nb.Contents())
		cb.Version++
		return v.handleBufferEvent(cb)
	}

	v.buffers[nb.Num] = nb
	if wf, ok := v.watchedFiles[nb.Name]; ok {
		// We are now picking up from a file that was previously watched. If we subsequently
		// close this buffer then we will handle that event and delete the entry in v.buffers
		// at which point the file watching will take back over again.
		delete(v.watchedFiles, nb.Name)
		nb.Version = wf.Version + 1
	} else {
		// first time we have seen the buffer
		if v.doIncrementalSync() {
			nb.Listener = v.ParseInt(v.ChannelCall("listener_add", v.Prefix()+string(config.FunctionEnrichDelta), nb.Num))
		}
		nb.Version = 0
	}
	return v.handleBufferEvent(nb)
}

// bufTextChanged is fired as a result of the TextChanged,TextChangedI autocmds; it is mutually
// exclusive with bufChanged
func (v *vimstate) bufTextChanged(args ...json.RawMessage) error {
	nb := v.currentBufferInfo(args[0])
	cb, ok := v.buffers[nb.Num]
	if !ok {
		return fmt.Errorf("have not seen buffer %v (%v) - this should be impossible", nb.Num, nb.Name)
	}
	cb.SetContents(nb.Contents())
	cb.Version++
	return v.handleBufferEvent(cb)
}

type bufChangedChange struct {
	Lnum  int      `json:"lnum"`
	Col   int      `json:"col"`
	Added int      `json:"added"`
	End   int      `json:"end"`
	Type  string   `json:"type"`
	Lines []string `json:"lines"`
}

// bufChanged is fired as a result of the listener_add callback for a buffer; it is mutually
// exclusive with bufTextChanged. args are:
//
// bufChanged(bufnr, start, end, added, changes)
//
func (v *vimstate) bufChanged(args ...json.RawMessage) (interface{}, error) {
	bufnr := v.ParseInt(args[0])
	b, ok := v.buffers[bufnr]
	if !ok {
		return nil, fmt.Errorf("failed to resolve buffer %v in bufChanged callback", bufnr)
	}
	var changes []bufChangedChange
	v.Parse(args[4], &changes)
	if len(changes) == 0 {
		v.Logf("bufChanged: no changes to apply for %v", b.Name)
		return nil, nil
	}

	contents := bytes.Split(b.Contents()[:len(b.Contents())-1], []byte("\n"))
	b.Version++

	params := &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: b.ToTextDocumentIdentifier(),
			Version:                float64(b.Version),
		},
	}
	for _, c := range changes {
		var newcontents [][]byte
		change := protocol.TextDocumentContentChangeEvent{
			Range: &protocol.Range{
				Start: protocol.Position{
					Line:      float64(c.Lnum - 1),
					Character: 0,
				},
			},
		}
		newcontents = append(newcontents, contents[:c.Lnum-1]...)
		for _, l := range c.Lines {
			newcontents = append(newcontents, []byte(l))
		}
		if len(c.Lines) > 0 {
			change.Text = strings.Join(c.Lines, "\n") + "\n"
		}
		newcontents = append(newcontents, contents[c.End-1:]...)
		change.Range.End = protocol.Position{
			Line:      float64(c.End - 1),
			Character: 0,
		}
		contents = newcontents
		params.ContentChanges = append(params.ContentChanges, change)
	}
	// add back trailing newline
	b.SetContents(append(bytes.Join(contents, []byte("\n")), '\n'))
	return nil, v.server.DidChange(context.Background(), params)
}

func (v *vimstate) handleBufferEvent(b *types.Buffer) error {
	if b.Version == 0 {
		params := &protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{
				LanguageID: "go",
				URI:        string(b.URI()),
				Version:    float64(b.Version),
				Text:       string(b.Contents()),
			},
		}
		err := v.server.DidOpen(context.Background(), params)
		return err
	}

	params := &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: b.ToTextDocumentIdentifier(),
			Version:                float64(b.Version),
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Text: string(b.Contents()),
			},
		},
	}
	err := v.server.DidChange(context.Background(), params)
	return err
}

func (v *vimstate) deleteCurrentBuffer(args ...json.RawMessage) error {
	currBufNr := v.ParseInt(args[0])
	cb, ok := v.buffers[currBufNr]
	if !ok {
		return fmt.Errorf("tried to remove buffer %v; but we have no record of it", currBufNr)
	}
	if v.doIncrementalSync() {
		v.ChannelCall("listener_remove", cb.Listener)
	}
	delete(v.buffers, cb.Num)
	params := &protocol.DidCloseTextDocumentParams{
		TextDocument: cb.ToTextDocumentIdentifier(),
	}
	if err := v.server.DidClose(context.Background(), params); err != nil {
		return fmt.Errorf("failed to call gopls.DidClose on %v: %v", cb.Name, err)
	}
	return nil
}
