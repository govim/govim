package main

import (
	"context"
	"fmt"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/types"
)

func (v *vimstate) fillStruct(flags govim.CommandFlags, args ...string) error {
	b, point, err := v.bufCursorPos()
	if err != nil {
		return fmt.Errorf("failed to determine cursor position: %v", err)
	}

	textDoc := b.ToTextDocumentIdentifier()
	params := &protocol.CodeActionParams{
		TextDocument: textDoc,
		Range:        protocol.Range{Start: point.ToPosition(), End: point.ToPosition()},
		Context: protocol.CodeActionContext{
			Only: []protocol.CodeActionKind{protocol.RefactorRewrite},
		},
	}

	codeActions, err := v.server.CodeAction(context.Background(), params)
	if err != nil {
		return fmt.Errorf("codeAction failed: %v", err)
	}

	if len(codeActions) == 0 {
		return nil
	}

	// TODO: revisit this logic when gopls enables us to distingush two different
	// code action responses.
	//
	// The response from gopls contain Commands, but gopls currently responds
	// with all code actions at the current line (instead of the exact range
	// passed as parameter).
	//
	// If there are more than one command returned, we can't apply them all since
	// each one of them contains unspecified parameters that are bound to current
	// version of the document.

	// The gopls ExecuteCommand is blocking, and gopls will call back to govim
	// using ApplyEdit that must be handled before the blocking is released.
	// Since fillstruct is ordered by the user (and the single threaded nature
	// of vim), we are effectively blocking ApplyEdit from modifying buffers.
	//
	// To prevent a deadlock, we create a channel that ApplyEdit can use pass
	// edits to this thread (if needed). And then call ExecuteCommand in a
	// separate goroutine so that this thread can go on updating buffers
	// until the ExecuteCommand is released. When it is, we implicitly know
	// that ApplyEdit has been processed.
	editsCh := make(chan applyEditCall)
	v.govimplugin.applyEditsLock.Lock()
	v.govimplugin.applyEditsCh = editsCh
	v.govimplugin.applyEditsLock.Unlock()
	done := make(chan struct{})

	var ecErr error
	v.tomb.Go(func() error {
		// We can only apply one command at the moment since they all target the same document
		// version. Let's go for the first one and let the user call fillstruct again if they
		// want to fill several structs on the same line.
		ca := codeActions[0]
		_, ecErr = v.server.ExecuteCommand(context.Background(), &protocol.ExecuteCommandParams{
			Command:                ca.Command.Command,
			Arguments:              ca.Command.Arguments,
			WorkDoneProgressParams: protocol.WorkDoneProgressParams{},
		})

		v.govimplugin.applyEditsLock.Lock()
		v.govimplugin.applyEditsCh = nil
		v.govimplugin.applyEditsLock.Unlock()
		close(done)
		return nil
	})

	for {
		select {
		case <-done:
			if ecErr != nil {
				return fmt.Errorf("executeCommand failed: %v", ecErr)
			}
			return nil
		case c := <-editsCh:
			res, err := v.applyWorkspaceEdit(c.params)
			c.responseCh <- applyEditResponse{res, err}
		}
	}
}

// applyEditCall represents a single LSP ApplyEdit call including a channel used
// to pass a response back.
type applyEditCall struct {
	params     *protocol.ApplyWorkspaceEditParams
	responseCh chan applyEditResponse
}

// applyEditResponse represents a LSP ApplyEdit response
type applyEditResponse struct {
	res *protocol.ApplyWorkspaceEditResponse
	err error
}

func (v *vimstate) applyWorkspaceEdit(params *protocol.ApplyWorkspaceEditParams) (*protocol.ApplyWorkspaceEditResponse, error) {
	res := &protocol.ApplyWorkspaceEditResponse{Applied: true}

	edits := make(map[*types.Buffer][]protocol.TextEdit)
	for _, dc := range params.Edit.DocumentChanges {
		// verify that the version of the edits matches a buffer
		var buf *types.Buffer
		for _, b := range v.buffers {
			if b.URI() != span.URI(dc.TextDocument.URI) {
				continue
			}

			if ev := dc.TextDocument.Version; ev > 0 && ev != b.Version {
				return nil, fmt.Errorf("got edits for buffer version %v, found matching buffer with version %v", ev, b.Version)
			}

			buf = b
		}

		if buf == nil {
			// TODO: we might get edits for files that we don't have open so we need to support that
			// as well. For fillstruct this isn't an issue since the user calls it within an open file.
			res.FailureReason = fmt.Sprintf("got edits for buffer %v, but didn't find it", dc.TextDocument.URI)
			res.Applied = false
			return res, nil
		}
		edits[buf] = append(edits[buf], dc.Edits...)
	}

	for b, e := range edits {
		if err := v.applyProtocolTextEdits(b, e); err != nil {
			res.FailureReason = err.Error()
			res.Applied = false
			return res, nil
		}
	}
	return res, nil
}
