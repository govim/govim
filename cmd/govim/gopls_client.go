package main

import (
	"context"

	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
)

var _ protocol.Client = (*driver)(nil)

func (d *driver) ShowMessage(context.Context, *protocol.ShowMessageParams) error {
	panic("not implemented yet")
}
func (d *driver) ShowMessageRequest(context.Context, *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	panic("not implemented yet")
}
func (d *driver) LogMessage(context.Context, *protocol.LogMessageParams) error {
	panic("not implemented yet")
}
func (d *driver) Telemetry(context.Context, interface{}) error {
	panic("not implemented yet")
}
func (d *driver) RegisterCapability(context.Context, *protocol.RegistrationParams) error {
	panic("not implemented yet")
}
func (d *driver) UnregisterCapability(context.Context, *protocol.UnregistrationParams) error {
	panic("not implemented yet")
}
func (d *driver) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	panic("not implemented yet")
}
func (d *driver) Configuration(context.Context, *protocol.ConfigurationParams) ([]interface{}, error) {
	panic("not implemented yet")
}
func (d *driver) ApplyEdit(context.Context, *protocol.ApplyWorkspaceEditParams) (bool, error) {
	panic("not implemented yet")
}

func (d *driver) PublishDiagnostics(ctxt context.Context, params *protocol.PublishDiagnosticsParams) error {
	d.Logf("PublishDiagnostics callback: %v", params)
	return nil
}
