// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsp

import (
	"bytes"
	"context"
	"fmt"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/jsonrpc2"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	errors "golang.org/x/xerrors"
)

func (s *Server) didOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	// Confirm that the file's language ID is related to Go.
	uri := span.NewURI(params.TextDocument.URI)
	snapshots, err := s.session.DidModifyFile(ctx, source.FileModification{
		URI:        uri,
		Action:     source.Open,
		Version:    params.TextDocument.Version,
		Text:       []byte(params.TextDocument.Text),
		LanguageID: params.TextDocument.LanguageID,
	})
	if err != nil {
		return err
	}
	snapshot, _, err := snapshotOf(s.session, uri, snapshots)
	if err != nil {
		return err
	}
	fh, err := snapshot.GetFile(ctx, uri)
	if err != nil {
		return err
	}
	// Always run diagnostics when a file is opened.
	return s.diagnose(snapshot, fh)
}

func (s *Server) didChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	uri := span.NewURI(params.TextDocument.URI)
	text, err := s.changedText(ctx, uri, params.ContentChanges)
	if err != nil {
		return err
	}
	snapshots, err := s.session.DidModifyFile(ctx, source.FileModification{
		URI:     uri,
		Action:  source.Change,
		Version: params.TextDocument.Version,
		Text:    text,
	})
	if err != nil {
		return err
	}
	snapshot, view, err := snapshotOf(s.session, uri, snapshots)
	if err != nil {
		return err
	}
	// Ideally, we should be able to specify that a generated file should be opened as read-only.
	// Tell the user that they should not be editing a generated file.
	if s.wasFirstChange(uri) && source.IsGenerated(ctx, view, uri) {
		s.client.ShowMessage(ctx, &protocol.ShowMessageParams{
			Message: fmt.Sprintf("Do not edit this file! %s is a generated file.", uri.Filename()),
			Type:    protocol.Warning,
		})
	}
	fh, err := snapshot.GetFile(ctx, uri)
	if err != nil {
		return err
	}
	// Always update diagnostics after a file change.
	return s.diagnose(snapshot, fh)
}

func (s *Server) didSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) error {
	c := source.FileModification{
		URI:     span.NewURI(params.TextDocument.URI),
		Action:  source.Save,
		Version: params.TextDocument.Version,
	}
	if params.Text != nil {
		c.Text = []byte(*params.Text)
	}
	_, err := s.session.DidModifyFile(ctx, c)
	return err
}

func (s *Server) didClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	_, err := s.session.DidModifyFile(ctx, source.FileModification{
		URI:     span.NewURI(params.TextDocument.URI),
		Action:  source.Close,
		Version: -1,
		Text:    nil,
	})
	return err
}

// snapshotOf returns the snapshot corresponding to the view for the given file URI.
func snapshotOf(session source.Session, uri span.URI, snapshots []source.Snapshot) (source.Snapshot, source.View, error) {
	view, err := session.ViewOf(uri)
	if err != nil {
		return nil, nil, err
	}
	for _, s := range snapshots {
		if s.View() == view {
			return s, view, nil
		}
	}
	return nil, nil, errors.Errorf("bestSnapshot: no snapshot for %s", uri)
}

func (s *Server) wasFirstChange(uri span.URI) bool {
	if s.changedFiles == nil {
		s.changedFiles = make(map[span.URI]struct{})
	}
	_, ok := s.changedFiles[uri]
	return ok
}

func (s *Server) changedText(ctx context.Context, uri span.URI, changes []protocol.TextDocumentContentChangeEvent) ([]byte, error) {
	if len(changes) == 0 {
		return nil, jsonrpc2.NewErrorf(jsonrpc2.CodeInternalError, "no content changes provided")
	}

	// Check if the client sent the full content of the file.
	// We accept a full content change even if the server expected incremental changes.
	if len(changes) == 1 && changes[0].Range == nil && changes[0].RangeLength == 0 {
		return []byte(changes[0].Text), nil
	}

	return s.applyIncrementalChanges(ctx, uri, changes)
}

func (s *Server) applyIncrementalChanges(ctx context.Context, uri span.URI, changes []protocol.TextDocumentContentChangeEvent) ([]byte, error) {
	content, _, err := s.session.GetFile(uri, source.UnknownKind).Read(ctx)
	if err != nil {
		return nil, jsonrpc2.NewErrorf(jsonrpc2.CodeInternalError, "file not found (%v)", err)
	}
	for _, change := range changes {
		// Make sure to update column mapper along with the content.
		converter := span.NewContentConverter(uri.Filename(), content)
		m := &protocol.ColumnMapper{
			URI:       uri,
			Converter: converter,
			Content:   content,
		}
		if change.Range == nil {
			return nil, jsonrpc2.NewErrorf(jsonrpc2.CodeInternalError, "unexpected nil range for change")
		}
		spn, err := m.RangeSpan(*change.Range)
		if err != nil {
			return nil, err
		}
		if !spn.HasOffset() {
			return nil, jsonrpc2.NewErrorf(jsonrpc2.CodeInternalError, "invalid range for content change")
		}
		start, end := spn.Start().Offset(), spn.End().Offset()
		if end < start {
			return nil, jsonrpc2.NewErrorf(jsonrpc2.CodeInternalError, "invalid range for content change")
		}
		var buf bytes.Buffer
		buf.Write(content[:start])
		buf.WriteString(change.Text)
		buf.Write(content[end:])
		content = buf.Bytes()
	}
	return content, nil
}
