package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"sync"

	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/types"
)

const (
	exprBufNew = `{"Num": eval(expand('<abuf>')), "Name": expand('<afile>') != "" ? fnamemodify(expand('<afile>'),':p') : ""}`
)

type bufNewDetails struct {
	Num  int
	Name string
}

// bufNew corresponds to the BufNew autocommand in Vim for all buffers.
// BufWipeout is the corresponding lifecycle event when a buffer is wiped out.
// We therefore use the existence of an entry in (*vimstate).buffers to
// correspond to the lifetime of a buffer. Subsequent lifecycle methods augment
// that buffer value.
//
// BufNew fires exactly when you would expect a new buffer to be created but
// also in a couple of surprising situations:
//
// 1. when saving an unnamed buffer with ':w file'; BufNew is fired for the new
// buffer (file), followed by BufWipeout of the old unnamed buffer, followed by
// BufWipeout of the new buffer (file), followed by another BufNew for the new
// buffer (file). It's unclear why this dance happens...
// 2. when setting a quickfix list that includes a file that does not have a
// corresponding buffer, a new buffer is created via BufNew named with the
// filename
func (v *vimstate) bufNew(args ...json.RawMessage) error {
	var nb bufNewDetails
	v.Parse(args[0], &nb)
	return v.bufNewImpl(nb)
}

func (v *vimstate) bufNewImpl(nb bufNewDetails) error {
	if _, ok := v.buffers[nb.Num]; ok {
		return fmt.Errorf("we already know about buffer %v; how is that possible?", nb.Num)
	}
	v.buffers[nb.Num] = &types.Buffer{Num: nb.Num, Name: nb.Name}
	return nil
}

const (
	exprBufWinEnter = `{"Num": eval(expand('<abuf>')), "Name": expand('<afile>') != "" ? fnamemodify(expand('<afile>'),':p') : "", "Contents": join(getbufline(eval(expand('<abuf>')), 0, "$"), "\n")."\n", "Loaded": bufloaded(eval(expand('<abuf>')))}`
)

type bufWinEnterDetails struct {
	Num      int
	Name     string
	Contents string
	Loaded   int
}

// bufWinEnter corresponds to the BufWinEnter autocommand in Vim for all buffers.
// It fires when a buffer is first loaded in a window, and therefore corresponds to
// a buffer being loaded. The corresponding end of lifecycle event is BufUnload
func (v *vimstate) bufWinEnter(args ...json.RawMessage) error {
	var nbinfo bufWinEnterDetails
	v.Parse(args[0], &nbinfo)
	return v.bufWinEnterImpl(nbinfo)
}

func (v *vimstate) bufWinEnterImpl(nbinfo bufWinEnterDetails) error {
	nb := types.NewBuffer(nbinfo.Num, nbinfo.Name, []byte(nbinfo.Contents), nbinfo.Loaded == 1)
	b, ok := v.buffers[nb.Num]
	if !ok {
		return fmt.Errorf("BufWinEnter fired for buffer %v; but we don't know about it", nb.Num)
	}
	if b.Loaded {
		// This happens when the buffer is already loaded in another window
		return nil
	}
	b.Loaded = true
	if b.Listener != 0 {
		return fmt.Errorf("we already have a listener for buffer %v; how is that possible?", b.Num)
	}
	b.Listener = v.ParseInt(v.ChannelCall("listener_add", v.Prefix()+string(config.FunctionEnrichDelta), b.Num))

	// If we load a buffer that already had diagnostics reported by gopls, the buffer number must be
	// updated to ensure that sign placement etc. works.
	diags := *v.diagnosticsCache
	for i, d := range diags {
		if d.Buf == -1 && d.Filename == nb.URI().Filename() {
			diags[i].Buf = b.Num
		}
	}

	if b.Name != nb.Name {
		return fmt.Errorf("BufWinEnter fired for buffer %v; but its name appears to have changed from %q to %q?", b.Num, b.Name, nb.Name)
	}

	b.SetContents(nb.Contents())
	b.Version++
	v.triggerBufferASTUpdate(b)
	if !bufferOfInterestToGopls(b) {
		return nil
	}
	if err := v.updateSigns(true); err != nil {
		v.Logf("failed to update signs for buffer %d: %v", nb.Num, err)
	}
	if err := v.redefineHighlights(true); err != nil {
		v.Logf("failed to update highlights for buffer %d: %v", nb.Num, err)
	}
	params := &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     protocol.DocumentURI(b.URI()),
			Version: float64(b.Version),
			Text:    string(b.Contents()),
		},
	}
	err := v.server.DidOpen(context.Background(), params)
	return err
}

// bufWritePre corresponds to the BufWritePre autocommand in Vim for all buffers.
func (v *vimstate) bufWritePre(args ...json.RawMessage) error {
	bufnr := v.ParseInt(args[0])
	b, ok := v.buffers[bufnr]
	if !ok {
		return fmt.Errorf("failed to resolve buffer %v", bufnr)
	}
	if !b.Loaded {
		// Because of https://github.com/vim/vim/issues/5655 we see a BufWrite
		// event for a buffer for which we have not seen a BufWinEnter when we
		// are writing an unnamed buffer to a file. So from govim's perspective
		// the buffer is not loaded. Hence we force load the buffer
		if v.strictVimBufferLifecycle() {
			return fmt.Errorf("saw BufWritePre event for buffer %v; but buffer was not loaded", bufnr)
		}
		var nbinfo bufWinEnterDetails
		v.Parse(v.ChannelExpr(exprBufWinEnter), &nbinfo)
		v.bufWinEnterImpl(nbinfo)
	}
	b, err := v.getLoadedBuffer(v.ParseInt(args[0]))
	if err != nil {
		return err
	}
	return v.formatCurrentBuffer(b)
}

// bufWritePost corresponds to the BufWritePost autocommand in Vim for all buffers.
func (v *vimstate) bufWritePost(args ...json.RawMessage) error {
	currBufNr := v.ParseInt(args[0])
	cb, ok := v.buffers[currBufNr]
	if !ok {
		return fmt.Errorf("tried to handle BufWritePost for buffer %v; but we have no record of it", currBufNr)
	}
	if !bufferOfInterestToGopls(cb) {
		return nil
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

// bufUnload corresponds to the BufUnload autocommand in Vim for all buffers.
// It fires when a buffer is unloaded. Its corresponding lifecycle event is
// BufWinEnter.
func (v *vimstate) bufUnload(args ...json.RawMessage) error {
	bufnr := v.ParseInt(args[0])
	return v.bufUnloadImpl(bufnr)
}

func (v *vimstate) bufUnloadImpl(bufnr int) error {
	b, ok := v.buffers[bufnr]
	if !ok {
		// Becuase of https://github.com/vim/vim/issues/5655 we might see a buffer
		// unload for a buffer that has already been wiped out. Just log and ignore
		// this case
		msg := fmt.Sprintf("BufUnload fired for buffer %v; but we don't know about it; ignoring", bufnr)
		if v.strictVimBufferLifecycle() {
			return fmt.Errorf(msg)
		}
		v.Logf(msg)
		return nil
	}
	b.Loaded = false

	// The diagnosticsCache is updated with -1 (unknown buffer) as bufnr.  We
	// don't want to remove the entries completely here since we want to show
	// them in the quickfix window. We don't need to remove existing text
	// properties or signs here since they are removed by vim automatically when
	// a buffer is unloaded and deleted respectively.
	diags := *v.diagnosticsCache
	for i := range diags {
		if diags[i].Buf == bufnr {
			diags[i].Buf = -1
		}
	}

	v.ChannelCall("listener_remove", b.Listener)
	b.Listener = 0

	if !bufferOfInterestToGopls(b) {
		return nil
	}
	params := &protocol.DidCloseTextDocumentParams{
		TextDocument: b.ToTextDocumentIdentifier(),
	}
	if err := v.server.DidClose(context.Background(), params); err != nil {
		return fmt.Errorf("failed to call gopls.DidClose on %v: %v", b.Name, err)
	}
	return nil
}

// bufDelete corresponds to the BufDelete autocommand in Vim for all buffers.
// BufDelete's corresponding lifecycle event is BufCreate. We use bufDelete
// to work around some Vim issues documented in the method body.
func (v *vimstate) bufDelete(args ...json.RawMessage) error {
	currBufNr := v.ParseInt(args[0])
	cb, ok := v.buffers[currBufNr]
	if !ok {
		return fmt.Errorf("tried to remove buffer %v; but we have no record of it", currBufNr)
	}

	// Becuase of https://github.com/vim/vim/issues/5655 we might see a buffer
	// that is still loaded. Unload that buffer first
	if cb.Loaded && !v.strictVimBufferLifecycle() {
		return v.bufUnloadImpl(currBufNr)
	}
	return nil
}

// bufWipeout corresponds to the BufWipeout autocommand in Vim for all buffers.
// Its corresponding lifecycle event is BufNew. See the bufNew method for more
// details.
func (v *vimstate) bufWipeout(args ...json.RawMessage) error {
	currBufNr := v.ParseInt(args[0])
	cb, ok := v.buffers[currBufNr]
	if !ok {
		// Because of https://github.com/vim/vim/issues/5656 we sometimes see a wipeout
		// that doesn't have a corresponding BufNew. Log and ignore
		msg := fmt.Sprintf("tried to wipeout buffer %v; but we have no record of it", currBufNr)
		if v.strictVimBufferLifecycle() {
			return fmt.Errorf(msg)
		}
		v.Logf(msg)
		return nil
	}
	delete(v.buffers, cb.Num)
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
	if !bufferOfInterestToGopls(b) {
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
	if !strings.HasSuffix(b.Name, ".go") {
		return
	}
	b.ASTWait = make(chan bool)
	v.bufferUpdates <- &bufferUpdate{
		buffer:   b,
		wait:     b.ASTWait,
		name:     b.Name,
		version:  b.Version,
		contents: b.Contents(),
	}
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

func bufferOfInterestToGopls(b *types.Buffer) bool {
	return strings.HasSuffix(b.Name, ".go") || filepath.Base(b.Name) == "go.mod"
}
