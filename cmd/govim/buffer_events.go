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

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/types"
	"github.com/kr/pretty"
)

// bufferStateChange fires in response to any change in Vim buffer state.  We
// only track a subset of buffer details and this is defined in
// types.BufferState
func (v *vimstate) bufferStateChange(args ...json.RawMessage) error {
	var updates []*types.BufferState
	changes := newBufferStateChanges(v)
	v.Parse(args[0], &updates)
	reverseLookup := make(map[int]*types.BufferState)
	for _, post := range updates {
		reverseLookup[post.Num] = post
		var pre *types.BufferState
		b, exists := v.buffers[post.Num]
		if !exists {
			b = post.ToBuffer()
			v.buffers[b.Num] = b
		} else {
			// Snapshot the current state and update the buffer state
			pre = snapState(b.BufferState)
			if *pre == *post {
				continue
			}
			b.BufferState = *post
		}
		changes.add(b, pre, post)
	}
	for _, b := range v.buffers {
		pre := snapState(b.BufferState)
		post, exists := reverseLookup[b.Num]
		if !exists {
			b.Loaded = false
			delete(v.buffers, b.Num)
		} else {
			if *pre == *post {
				continue
			}
			b.BufferState = *post
		}
		changes.add(b, pre, post)
	}
	for _, ch := range changes.order {
		changes.bufferStateChange(ch.buffer, ch.pre, ch.post)
	}
	return nil
}

// bufRead corresponds to the BufRead autocommand in Vim for all buffers.
//
// TODO there is an inefficiency when it comes to loading a buffer from a file
// for the first time, e.g. vim main.go. Firstly we get a BufferStateChange
// event that indicates the buffer is in a fileready state off the back of a
// BufRead event.  Then (and by definition it occurs after because the BufRead
// autocommand that triggers a BufferStateChange event is registered first) we
// get a callback into bufRead, also as a result of the BufRead event. In both
// situations we ensure the govim copy of the buffer is current. The
// inefficiency comes about because when the file is being loaded in a buffer
// for the first time, the call to bufRead actually does nothing - the govim
// copy of the buffer contents is already current because of the buffer having
// just become fileready. Once the buffer life-cycle changes stabilise we can
// look to optimise this.
func (v *vimstate) bufRead(args ...json.RawMessage) error {
	bufnr := v.ParseInt(args[0])
	b, err := v.getLoadedBuffer(bufnr)
	if err != nil {
		return err
	}
	contents := v.ParseString(v.ChannelExprf(`join(getbufline(%v, 0, "$"), "\n")."\n"`, b.Num))
	contentBytes := []byte(contents)
	if bytes.Equal(contentBytes, b.Contents()) {
		return nil
	}
	b.SetContents(contentBytes)
	b.Version++
	params := &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: b.ToTextDocumentIdentifier(),
			Version:                float64(b.Version),
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Text: string(contents),
			},
		},
	}
	return v.server.DidChange(context.Background(), params)
}

// bufWritePre corresponds to the BufWritePre autocommand in Vim for all buffers.
func (v *vimstate) bufWritePre(args ...json.RawMessage) error {
	bufnr := v.ParseInt(args[0])
	b, err := v.getLoadedBuffer(bufnr)
	if err != nil {
		return err
	}
	return v.formatCurrentBuffer(b)
}

// bufWritePost corresponds to the BufWritePost autocommand in Vim for all buffers.
func (v *vimstate) bufWritePost(args ...json.RawMessage) error {
	bufnr := v.ParseInt(args[0])
	b, err := v.getLoadedBuffer(bufnr)
	if err != nil {
		return err
	}
	if !b.IsOfGoplsInterest() {
		return nil
	}
	params := &protocol.DidSaveTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: b.ToTextDocumentIdentifier(),
			Version:                float64(b.Version),
		},
	}
	if err := v.server.DidSave(context.Background(), params); err != nil {
		return fmt.Errorf("failed to call gopls.DidSave on %v: %v", b.Name, err)
	}
	return nil
}

type bufChangedChange struct {
	Lnum  int      `json:"lnum"`
	Col   int      `json:"col"`
	Added int      `json:"added"`
	End   int      `json:"end"`
	Type  string   `json:"type"`
	Lines []string `json:"lines"`
}

// bufChanged is fired as a result of the listener_add callback for a buffer. Args are:
//
//     bufChanged(bufnr, start, end, added, changes)
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
		v.Logf(">> processing %v", pretty.Sprint(c))
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
	if !b.IsOfGoplsInterest() {
		return nil, nil
	}
	return nil, v.server.DidChange(context.Background(), params)
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
				// This is best efforts
				f, _ := parser.ParseFile(fset, upd.name, upd.contents, parser.AllErrors)
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

func (v *vimstate) clearBufferAST(b *types.Buffer) {
	b.ASTWait = nil
	b.AST = nil
	b.Fset = nil
}

// getLoadedBuffer returns the loaded buffer identified by the buffer number
// bufnr, else it returns an error indicating either that the buffer could not
// be resolved or that the buffer is not loaded.
func (v *vimstate) getLoadedBuffer(bufnr int) (*types.Buffer, error) {
	b, ok := v.buffers[bufnr]
	if !ok {
		return nil, fmt.Errorf("failed to resolve buffer %v", bufnr)
	}
	if !b.Loaded {
		return nil, fmt.Errorf("buffer %v is not loaded", bufnr)
	}
	return b, nil
}
