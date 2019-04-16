package main

import (
	"context"

	"github.com/kr/pretty"
	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/span"
)

var _ protocol.Client = (*govimplugin)(nil)

func (g *govimplugin) ShowMessage(context.Context, *protocol.ShowMessageParams) error {
	panic("not implemented yet")
}
func (g *govimplugin) ShowMessageRequest(context.Context, *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	panic("not implemented yet")
}
func (g *govimplugin) LogMessage(ctxt context.Context, params *protocol.LogMessageParams) error {
	g.logGoplsClientf("LogMessage callback: %v", pretty.Sprint(params))
	return nil
}
func (g *govimplugin) Telemetry(context.Context, interface{}) error {
	panic("not implemented yet")
}
func (g *govimplugin) RegisterCapability(context.Context, *protocol.RegistrationParams) error {
	panic("not implemented yet")
}
func (g *govimplugin) UnregisterCapability(context.Context, *protocol.UnregistrationParams) error {
	panic("not implemented yet")
}
func (g *govimplugin) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	panic("not implemented yet")
}
func (g *govimplugin) Configuration(context.Context, *protocol.ConfigurationParams) ([]interface{}, error) {
	panic("not implemented yet")
}
func (g *govimplugin) ApplyEdit(context.Context, *protocol.ApplyWorkspaceEditParams) (bool, error) {
	panic("not implemented yet")
}

func (g *govimplugin) PublishDiagnostics(ctxt context.Context, params *protocol.PublishDiagnosticsParams) error {
	g.logGoplsClientf("PublishDiagnostics callback: %v", pretty.Sprint(params))
	g.Schedule(func(govim.Govim) error {
		v := g.vimstate
		// TODO improve the efficiency of this. When we are watching files not yet loaded
		// in Vim, we will likely have a reference to the file in vimstate. At this point
		// we will need a lookup from URI -> Buffer which might give us nothing, at which
		// point we fall back to the file. For now we simply check buffers first, then fall
		// back to loading the file from disk

		v.diagnostics[span.URI(params.URI)] = params.Diagnostics
		v.diagnosticsChanged = true

		return nil
	})
	return nil
}

func (g *govimplugin) logGoplsClientf(format string, args ...interface{}) {
	if format[len(format)-1] != '\n' {
		format = format + "\n"
	}
	g.Logf("gopls client start =======================\n"+format+"gopls client end =======================\n", args...)
}
