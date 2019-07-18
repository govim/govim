package main

import (
	"context"

	"github.com/kr/pretty"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
)

type loggingGoplsServer struct {
	u protocol.Server
	g *govimplugin
}

var _ protocol.Server = loggingGoplsServer{}

func (l loggingGoplsServer) Logf(format string, args ...interface{}) {
	if format[len(format)-1] != '\n' {
		format = format + "\n"
	}
	l.g.Logf("gopls server start =======================\n"+format+"gopls server end =======================\n", args...)
}

func (l loggingGoplsServer) Initialize(ctxt context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	l.Logf("gopls.Initialize() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.Initialize(ctxt, params)
	l.Logf("gopls.Initialize() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) Initialized(ctxt context.Context, params *protocol.InitializedParams) error {
	l.Logf("gopls.Initialized() call; params:\n%v", pretty.Sprint(params))
	err := l.u.Initialized(ctxt, params)
	l.Logf("gopls.Initialized() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) Shutdown(ctxt context.Context) error {
	l.Logf("gopls.Shutdown() call")
	err := l.u.Shutdown(ctxt)
	l.Logf("gopls.Shutdown() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) Exit(ctxt context.Context) error {
	l.Logf("gopls.Exit() call")
	err := l.u.Exit(ctxt)
	l.Logf("gopls.Exit() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) DidChangeWorkspaceFolders(ctxt context.Context, params *protocol.DidChangeWorkspaceFoldersParams) error {
	l.Logf("gopls.DidChangeWorkspaceFolders() call; params:\n%v", pretty.Sprint(params))
	err := l.u.DidChangeWorkspaceFolders(ctxt, params)
	l.Logf("gopls.DidChangeWorkspaceFolders() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) DidChangeConfiguration(ctxt context.Context, params *protocol.DidChangeConfigurationParams) error {
	l.Logf("gopls.DidChangeConfiguration() call; params:\n%v", pretty.Sprint(params))
	err := l.u.DidChangeConfiguration(ctxt, params)
	l.Logf("gopls.DidChangeConfiguration() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) DidChangeWatchedFiles(ctxt context.Context, params *protocol.DidChangeWatchedFilesParams) error {
	l.Logf("gopls.DidChangeWatchedFiles() call; params:\n%v", pretty.Sprint(params))
	err := l.u.DidChangeWatchedFiles(ctxt, params)
	l.Logf("gopls.DidChangeWatchedFiles() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) Symbol(ctxt context.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	l.Logf("gopls.Symbol() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.Symbol(ctxt, params)
	l.Logf("gopls.Symbol() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) ExecuteCommand(ctxt context.Context, params *protocol.ExecuteCommandParams) (interface{}, error) {
	l.Logf("gopls.ExecuteCommand() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.ExecuteCommand(ctxt, params)
	l.Logf("gopls.ExecuteCommand() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) DidOpen(ctxt context.Context, params *protocol.DidOpenTextDocumentParams) error {
	l.Logf("gopls.DidOpen() call; params:\n%v", pretty.Sprint(params))
	err := l.u.DidOpen(ctxt, params)
	l.Logf("gopls.DidOpen() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) DidChange(ctxt context.Context, params *protocol.DidChangeTextDocumentParams) error {
	l.Logf("gopls.DidChange() call; params:\n%v", pretty.Sprint(params))
	err := l.u.DidChange(ctxt, params)
	l.Logf("gopls.DidChange() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) WillSave(ctxt context.Context, params *protocol.WillSaveTextDocumentParams) error {
	l.Logf("gopls.WillSave() call; params:\n%v", pretty.Sprint(params))
	err := l.u.WillSave(ctxt, params)
	l.Logf("gopls.WillSave() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) WillSaveWaitUntil(ctxt context.Context, params *protocol.WillSaveTextDocumentParams) ([]protocol.TextEdit, error) {
	l.Logf("gopls.WillSaveWaitUntil() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.WillSaveWaitUntil(ctxt, params)
	l.Logf("gopls.WillSaveWaitUntil() return; err: %v; res\n", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) DidSave(ctxt context.Context, params *protocol.DidSaveTextDocumentParams) error {
	l.Logf("gopls.DidSave() call; params:\n%v", pretty.Sprint(params))
	err := l.u.DidSave(ctxt, params)
	l.Logf("gopls.DidSave() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) DidClose(ctxt context.Context, params *protocol.DidCloseTextDocumentParams) error {
	l.Logf("gopls.DidClose() call; params:\n%v", pretty.Sprint(params))
	err := l.u.DidClose(ctxt, params)
	l.Logf("gopls.DidClose() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) Completion(ctxt context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	l.Logf("gopls.Completion() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.Completion(ctxt, params)
	l.Logf("gopls.Completion() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) Resolve(ctxt context.Context, params *protocol.CompletionItem) (*protocol.CompletionItem, error) {
	l.Logf("gopls.Resolve() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.Resolve(ctxt, params)
	l.Logf("gopls.Resolve() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) Hover(ctxt context.Context, params *protocol.TextDocumentPositionParams) (*protocol.Hover, error) {
	l.Logf("gopls.Hover() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.Hover(ctxt, params)
	l.Logf("gopls.Hover() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) SignatureHelp(ctxt context.Context, params *protocol.TextDocumentPositionParams) (*protocol.SignatureHelp, error) {
	l.Logf("gopls.SignatureHelp() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.SignatureHelp(ctxt, params)
	l.Logf("gopls.SignatureHelp() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) Definition(ctxt context.Context, params *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	l.Logf("gopls.Definition() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.Definition(ctxt, params)
	l.Logf("gopls.Definition() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) TypeDefinition(ctxt context.Context, params *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	l.Logf("gopls.TypeDefinition() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.TypeDefinition(ctxt, params)
	l.Logf("gopls.TypeDefinition() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) Implementation(ctxt context.Context, params *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	l.Logf("gopls.Implementation() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.Implementation(ctxt, params)
	l.Logf("gopls.Implementation() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) References(ctxt context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	l.Logf("gopls.References() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.References(ctxt, params)
	l.Logf("gopls.References() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) DocumentHighlight(ctxt context.Context, params *protocol.TextDocumentPositionParams) ([]protocol.DocumentHighlight, error) {
	l.Logf("gopls.DocumentHighlight() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.DocumentHighlight(ctxt, params)
	l.Logf("gopls.DocumentHighlight() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) DocumentSymbol(ctxt context.Context, params *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error) {
	l.Logf("gopls.DocumentSymbol() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.DocumentSymbol(ctxt, params)
	l.Logf("gopls.DocumentSymbol() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) CodeAction(ctxt context.Context, params *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	l.Logf("gopls.CodeAction() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.CodeAction(ctxt, params)
	l.Logf("gopls.CodeAction() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) CodeLens(ctxt context.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	l.Logf("gopls.CodeLens() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.CodeLens(ctxt, params)
	l.Logf("gopls.CodeLens() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) ResolveCodeLens(ctxt context.Context, params *protocol.CodeLens) (*protocol.CodeLens, error) {
	l.Logf("gopls.ResolveCodeLens() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.ResolveCodeLens(ctxt, params)
	l.Logf("gopls.ResolveCodeLens() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) DocumentLink(ctxt context.Context, params *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	l.Logf("gopls.DocumentLink() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.DocumentLink(ctxt, params)
	l.Logf("gopls.DocumentLink() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) ResolveDocumentLink(ctxt context.Context, params *protocol.DocumentLink) (*protocol.DocumentLink, error) {
	l.Logf("gopls.ResolveDocumentLink() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.ResolveDocumentLink(ctxt, params)
	l.Logf("gopls.ResolveDocumentLink() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) DocumentColor(ctxt context.Context, params *protocol.DocumentColorParams) ([]protocol.ColorInformation, error) {
	l.Logf("gopls.DocumentColor() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.DocumentColor(ctxt, params)
	l.Logf("gopls.DocumentColor() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) ColorPresentation(ctxt context.Context, params *protocol.ColorPresentationParams) ([]protocol.ColorPresentation, error) {
	l.Logf("gopls.ColorPresentation() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.ColorPresentation(ctxt, params)
	l.Logf("gopls.ColorPresentation() return; err: %v; res:\n%v", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) Formatting(ctxt context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	l.Logf("gopls.Formatting() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.Formatting(ctxt, params)
	l.Logf("gopls.Formatting() return; err: %v; res:\n%v\n", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) RangeFormatting(ctxt context.Context, params *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	l.Logf("gopls.RangeFormatting() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.RangeFormatting(ctxt, params)
	l.Logf("gopls.RangeFormatting() return; err: %v; res\n", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) OnTypeFormatting(ctxt context.Context, params *protocol.DocumentOnTypeFormattingParams) ([]protocol.TextEdit, error) {
	l.Logf("gopls.OnTypeFormatting() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.OnTypeFormatting(ctxt, params)
	l.Logf("gopls.OnTypeFormatting() return; err: %v; res\n", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) Rename(ctxt context.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	l.Logf("gopls.Rename() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.Rename(ctxt, params)
	l.Logf("gopls.Rename() return; err: %v; res\n", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) FoldingRange(ctxt context.Context, params *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	l.Logf("gopls.FoldingRange() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.FoldingRange(ctxt, params)
	l.Logf("gopls.FoldingRange() return; err: %v; res\n", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) Declaration(ctxt context.Context, params *protocol.TextDocumentPositionParams) ([]protocol.DeclarationLink, error) {
	l.Logf("gopls.Declaration() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.Declaration(ctxt, params)
	l.Logf("gopls.Declaration() return; err: %v; res\n", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) LogTraceNotification(ctxt context.Context, params *protocol.LogTraceParams) error {
	l.Logf("gopls.LogTraceNotification() call; params:\n%v", pretty.Sprint(params))
	err := l.u.LogTraceNotification(ctxt, params)
	l.Logf("gopls.LogTraceNotification() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) PrepareRename(ctxt context.Context, params *protocol.TextDocumentPositionParams) (*protocol.Range, error) {
	l.Logf("gopls.PrepareRename() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.PrepareRename(ctxt, params)
	l.Logf("gopls.PrepareRename() return; err: %v; res\n", err, pretty.Sprint(res))
	return res, err
}

func (l loggingGoplsServer) SetTraceNotification(ctxt context.Context, params *protocol.SetTraceParams) error {
	l.Logf("gopls.SetTraceNotification() call; params:\n%v", pretty.Sprint(params))
	err := l.u.SetTraceNotification(ctxt, params)
	l.Logf("gopls.SetTraceNotification() return; err: %v", err)
	return err
}

func (l loggingGoplsServer) SelectionRange(ctxt context.Context, params *protocol.SelectionRangeParams) ([]protocol.SelectionRange, error) {
	l.Logf("gopls.SelectionRange() call; params:\n%v", pretty.Sprint(params))
	res, err := l.u.SelectionRange(ctxt, params)
	l.Logf("gopls.SelectionRange() return; err: %v; res\n", err, pretty.Sprint(res))
	return res, err
}
