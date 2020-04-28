package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"strings"
	"sync"

	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/types"
)

func (v *vimstate) bufReadPost(args ...json.RawMessage) error {
	nb := v.currentBufferInfo(args[0])

	// If we load a buffer that already had diagnostics reported by gopls, the buffer number must be
	// updated to ensure that sign placement etc. works.
	diags := *v.diagnosticsCache
	for i, d := range diags {
		if d.Buf == -1 && d.Filename == nb.URI().Filename() {
			diags[i].Buf = nb.Num
		}
	}

	if cb, ok := v.buffers[nb.Num]; ok {
		// reload of buffer, e.v. e!
		cb.Loaded = nb.Loaded

		// If the contents are the same we probably just re-loaded a currently
		// unloaded buffer.  We shouldn't increase version in that case, but we
		// have to re-place signs and redefine highlights since text properties
		// are removed when a buffer is unloaded.
		if bytes.Equal(nb.Contents(), cb.Contents()) {
			if err := v.updateSigns(true); err != nil {
				v.Logf("failed to update signs for buffer %d: %v", nb.Num, err)
			}
			if err := v.redefineHighlights(true); err != nil {
				v.Logf("failed to update highlights for buffer %d: %v", nb.Num, err)
			}
			return nil
		}
		cb.SetContents(nb.Contents())
		cb.Version++
		return v.handleBufferEvent(cb)
	}

	v.buffers[nb.Num] = nb
	nb.Version = 1
	nb.Listener = v.ParseInt(v.ChannelCall("listener_add", v.Prefix()+string(config.FunctionEnrichDelta), nb.Num))

	if err := v.updateSigns(true); err != nil {
		v.Logf("failed to update signs for buffer %d: %v", nb.Num, err)
	}

	if err := v.redefineHighlights(true); err != nil {
		v.Logf("failed to update highlights for buffer %d: %v", nb.Num, err)
	}

	return v.handleBufferEvent(nb)
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
	// For now, if we are "manually" highlighting, any change (in a .go file)
	// causes an existing highlights to be removed.
	if v.highlightingReferences {
		v.highlightingReferences = false
		v.removeReferenceHighlight(nil)
	}

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
	v.triggerBufferASTUpdate(b)
	if err := v.server.DidChange(context.Background(), params); err != nil {
		return nil, fmt.Errorf("failed to notify gopls of change: %v", err)
	}
	return nil, nil
}

func (v *vimstate) bufUnload(args ...json.RawMessage) error {
	bufnr := v.ParseInt(args[0])
	if _, ok := v.buffers[bufnr]; !ok {
		return nil
	}
	v.buffers[bufnr].Loaded = false
	return nil
}

func (v *vimstate) handleBufferEvent(b *types.Buffer) error {
	v.triggerBufferASTUpdate(b)
	if b.Version == 1 {
		params := &protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{
				LanguageID: "go",
				URI:        protocol.DocumentURI(b.URI()),
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

	// The diagnosticsCache is updated with -1 (unknown buffer) as bufnr.
	// We don't want to remove the entries completely here since we want to show them in
	// the quickfix window. And we don't need to remove existing signs or text properties
	// either here since they are removed by vim automatically when a buffer is deleted.
	diags := *v.diagnosticsCache
	for i, d := range diags {
		if d.Buf == currBufNr {
			diags[i].Buf = -1
		}
	}

	v.ChannelCall("listener_remove", cb.Listener)
	delete(v.buffers, cb.Num)
	params := &protocol.DidCloseTextDocumentParams{
		TextDocument: cb.ToTextDocumentIdentifier(),
	}
	if err := v.server.DidClose(context.Background(), params); err != nil {
		return fmt.Errorf("failed to call gopls.DidClose on %v: %v", cb.Name, err)
	}
	return nil
}

func (v *vimstate) bufWritePost(args ...json.RawMessage) error {
	currBufNr := v.ParseInt(args[0])
	cb, ok := v.buffers[currBufNr]
	if !ok {
		return fmt.Errorf("tried to handle BufWritePost for buffer %v; but we have no record of it", currBufNr)
	}
	params := &protocol.DidSaveTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: cb.ToTextDocumentIdentifier(),
			Version:                float64(cb.Version),
		},
	}
	if err := v.server.DidSave(context.Background(), params); err != nil {
		return fmt.Errorf("failed to call gopls.DidSave on %v: %v", cb.Name, err)
	}
	return nil
}

type bufferUpdate struct {
	buffer   *types.Buffer
	wait     chan bool
	name     string
	version  int
	contents []byte
}

func (g *govimplugin) startProcessBufferUpdates() {
	g.bufferUpdates = make(chan *bufferUpdate)
	g.tomb.Go(func() error {
		latest := make(map[*types.Buffer]int)
		var lock sync.Mutex
		for upd := range g.bufferUpdates {
			upd := upd
			lock.Lock()
			latest[upd.buffer] = upd.version
			lock.Unlock()

			// Note we are not restricting the number of concurrent parses here.
			// This is simply because we are unlikely to ever get a sufficiently
			// high number of concurrent updates from Vim to make this necessary.
			// Like the Vim <-> govim <-> gopls "channel" would get
			// flooded/overloaded first
			g.tomb.Go(func() error {
				fset := token.NewFileSet()
				f, err := parser.ParseFile(fset, upd.name, upd.contents, parser.AllErrors)
				if err != nil {
					// This is best efforts so we just log the error as an info
					// message
					g.Logf("info only: failed to parse buffer %v: %v", upd.name, err)
				}
				lock.Lock()
				if latest[upd.buffer] == upd.version {
					upd.buffer.Fset = fset
					upd.buffer.AST = f
					delete(latest, upd.buffer)
				}
				lock.Unlock()
				close(upd.wait)
				return nil
			})
		}
		return nil
	})
}

func (v *vimstate) triggerBufferASTUpdate(b *types.Buffer) {
	b.ASTWait = make(chan bool)
	v.bufferUpdates <- &bufferUpdate{
		buffer:   b,
		wait:     b.ASTWait,
		name:     b.Name,
		version:  b.Version,
		contents: b.Contents(),
	}
}
