package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/types"
)

func (v *vimstate) bufReadPost(args ...json.RawMessage) error {
	b := v.currentBufferInfo(args[0])
	if cb, ok := v.buffers[b.Num]; ok {
		// reload of buffer, e.v. e!
		b.Version = cb.Version + 1
	} else if wf, ok := v.watchedFiles[b.Name]; ok {
		// We are now picking up from a file that was previously watched. If we subsequently
		// close this buffer then we will handle that event and delete the entry in v.buffers
		// at which point the file watching will take back over again.
		delete(v.watchedFiles, b.Name)
		b.Version = wf.Version + 1
	} else {
		b.Version = 0
	}
	return v.handleBufferEvent(b)
}

func (v *vimstate) bufTextChanged(args ...json.RawMessage) error {
	b := v.currentBufferInfo(args[0])
	cb, ok := v.buffers[b.Num]
	if !ok {
		return fmt.Errorf("have not seen buffer %v (%v) - this should be impossible", b.Num, b.Name)
	}
	b.Version = cb.Version + 1
	return v.handleBufferEvent(b)
}

func (v *vimstate) handleBufferEvent(b *types.Buffer) error {
	v.buffers[b.Num] = b

	if b.Version == 0 {
		params := &protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{
				URI:     string(b.URI()),
				Version: float64(b.Version),
				Text:    string(b.Contents),
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
				Text: string(b.Contents),
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
	delete(v.buffers, cb.Num)
	params := &protocol.DidCloseTextDocumentParams{
		TextDocument: cb.ToTextDocumentIdentifier(),
	}
	if err := v.server.DidClose(context.Background(), params); err != nil {
		return fmt.Errorf("failed to call gopls.DidClose on %v: %v", cb.Name, err)
	}
	return nil
}
