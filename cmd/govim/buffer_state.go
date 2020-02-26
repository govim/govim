package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/types"
)

type bufferStateChange struct {
	buffer *types.Buffer
	pre    *types.BufferState
	post   *types.BufferState
}

type bufferStateChanges struct {
	*vimstate

	order  []*bufferStateChange
	lookup map[int]*bufferStateChange
}

func newBufferStateChanges(v *vimstate) *bufferStateChanges {
	return &bufferStateChanges{
		vimstate: v,
		lookup:   make(map[int]*bufferStateChange),
	}
}

func (c *bufferStateChanges) add(b *types.Buffer, pre, post *types.BufferState) {
	ch := &bufferStateChange{
		buffer: b,
		pre:    pre,
		post:   post,
	}
	c.order = append(c.order, ch)
	c.lookup[b.Num] = ch
}

func snapState(bs types.BufferState) *types.BufferState {
	return &bs
}

// bufferStateChange is fired for each buffer that has a state change.
//
// TODO look at the error handling again here. If we get an error during any of
// this processing it's hard to imagine how we can/should recover because of
// the bad state we leave things in. At least returning the error makes it clear
// to the user something has gone wrong
func (c *bufferStateChanges) bufferStateChange(b *types.Buffer, pre, post *types.BufferState) error {
	// loaded?
	if !pre.IsLoaded() && post.IsLoaded() {
		if err := c.bufferNowLoaded(b, pre, post); err != nil {
			return err
		}
	}
	if pre.IsLoaded() && !post.IsLoaded() {
		if err := c.bufferNowNotLoaded(b, pre, post); err != nil {
			return err
		}
	}
	// fileready?
	if !pre.IsFileReady() && post.IsFileReady() {
		if err := c.bufferNowFileReady(b, pre, post); err != nil {
			return err
		}
	}
	if pre.IsFileReady() && !post.IsFileReady() {
		if err := c.bufferNowNotFileReady(b, pre, post); err != nil {
			return err
		}
	}
	// gopls interest?
	if !pre.IsOfGoplsInterest() && post.IsOfGoplsInterest() {
		if err := c.bufferNowOfGoplsInterest(b, pre, post); err != nil {
			return err
		}
	}
	if pre.IsOfGoplsInterest() && !post.IsOfGoplsInterest() {
		if err := c.bufferNowNotOfGoplsInterest(b, pre, post); err != nil {
			return err
		}
	}
	// .go AST parsing interest?
	if post.IsOfGoASTInterest() {
		c.triggerBufferASTUpdate(b)
	}
	if pre.IsOfGoASTInterest() && !post.IsOfGoASTInterest() {
		c.clearBufferAST(b)
	}
	return nil
}

// bufferNowLoaded is called whenever a buffer is now loaded. This can be a new
// buffer that is already loaded, or a buffer that moves to the loaded state.
//
// You must not doing anything in this callback that would trigger a new buffer
// lifecycle event
func (c *bufferStateChanges) bufferNowLoaded(b *types.Buffer, pre, post *types.BufferState) error {
	diags := *c.diagnosticsCache
	for i, d := range diags {
		if d.Buf == -1 && d.Filename == b.URI().Filename() {
			diags[i].Buf = b.Num
		}
	}
	b.Listener = c.ParseInt(c.ChannelCall("listener_add", c.Prefix()+string(config.FunctionEnrichDelta), b.Num))
	return nil
}

// bufferNowFileReady is called whenever a buffer is now "file ready". This is
// a govim-defined term to indicate that a named buffer has been read from a
// file or that the buffer represents a new file.
//
// You must not doing anything in this callback that would trigger a new buffer
// lifecycle event
func (c *bufferStateChanges) bufferNowFileReady(b *types.Buffer, pre, post *types.BufferState) error {
	contents := c.ParseString(c.ChannelExprf(`join(getbufline(%v, 0, "$"), "\n")."\n"`, b.Num))
	contentBytes := []byte(contents)
	if bytes.Equal(contentBytes, b.Contents()) {
		return nil
	}
	b.SetContents(contentBytes)
	b.Version++
	return nil
}

// bufferNowNotFileReady is called whenever a buffer is no longer "file ready".
// See comment for bufferNowFileReady.
//
// You must not doing anything in this callback that would trigger a new buffer
// lifecycle event
func (c *bufferStateChanges) bufferNowNotFileReady(b *types.Buffer, pre, post *types.BufferState) error {
	return nil
}

// bufferNowNotLoaded is called whenever a buffer is now not loaded. This can
// be a buffer that is now wiped out, or a buffer that is simply unloaded.
//
// You must not doing anything in this callback that would trigger a new buffer
// lifecycle event
func (c *bufferStateChanges) bufferNowNotLoaded(b *types.Buffer, pre, post *types.BufferState) error {
	diags := *c.diagnosticsCache
	for i := range diags {
		if diags[i].Buf == b.Num {
			diags[i].Buf = -1
		}
	}
	c.ChannelCall("listener_remove", b.Listener)
	b.Listener = 0
	return nil
}

// bufferNowOfGoplsInterest is called whenever a buffer becomes of interest to
// gopls. "Of interest" buffers are those that are loaded and of the right file
// type.
//
// You must not doing anything in this callback that would trigger a new buffer
// lifecycle event
func (c *bufferStateChanges) bufferNowOfGoplsInterest(b *types.Buffer, pre, post *types.BufferState) error {
	// TODO move this to another method
	c.triggerBufferASTUpdate(b)

	if err := c.updateSigns(true); err != nil {
		c.Logf("failed to update signs for buffer %d: %v", b.Num, err)
	}
	if err := c.redefineHighlights(true); err != nil {
		c.Logf("failed to update highlights for buffer %d: %v", b.Num, err)
	}
	params := &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     protocol.DocumentURI(b.URI()),
			Version: float64(b.Version),
			Text:    string(b.Contents()),
		},
	}
	if err := c.server.DidOpen(context.Background(), params); err != nil {
		return err
	}
	return nil
}

// bufferNowNotOfGoplsInterest is called whenever a buffer becomes of interest to
// gopls. "Of interest" buffers are those that are loaded and of the right file
// type.
//
// You must not doing anything in this callback that would trigger a new buffer
// lifecycle event
func (c *bufferStateChanges) bufferNowNotOfGoplsInterest(b *types.Buffer, pre, post *types.BufferState) error {
	params := &protocol.DidCloseTextDocumentParams{
		TextDocument: b.ToTextDocumentIdentifier(),
	}
	if err := c.server.DidClose(context.Background(), params); err != nil {
		return fmt.Errorf("failed to call gopls.DidClose on %v: %v", b.Name, err)
	}
	return nil
}
