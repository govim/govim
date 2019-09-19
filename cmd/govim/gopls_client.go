package main

import (
	"context"
	"fmt"
	"reflect"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/kr/pretty"
)

const (
	goplsConfigNoDocsOnHover   = "noDocsOnHover"
	goplsConfigHoverKind       = "hoverKind"
	goplsDisableDeepCompletion = "disableDeepCompletion"
	goplsDisableFuzzyMatching  = "disableFuzzyMatching"
)

var _ protocol.Client = (*govimplugin)(nil)

func (g *govimplugin) ShowMessage(ctxt context.Context, params *protocol.ShowMessageParams) error {
	g.logGoplsClientf("ShowMessage callback: %v", params.Message)
	if params.Type != protocol.Error {
		return nil
	}

	g.Schedule(func(govim.Govim) error {
		opts := make(map[string]interface{})
		opts["mousemoved"] = "any"
		opts["moved"] = "any"
		opts["padding"] = []int{0, 1, 0, 1}
		opts["wrap"] = false
		opts["border"] = []int{}
		opts["highlight"] = "ErrorMsg"
		opts["line"] = 1
		opts["close"] = "click"

		g.ChannelCall("popup_create", params.Message, opts)
		return nil
	})
	return nil
}

func (g *govimplugin) ShowMessageRequest(context.Context, *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	panic("not implemented yet")
}

func (g *govimplugin) LogMessage(ctxt context.Context, params *protocol.LogMessageParams) error {
	defer absorbShutdownErr()
	g.logGoplsClientf("LogMessage callback: %v", pretty.Sprint(params))
	return nil
}

func (g *govimplugin) Telemetry(context.Context, interface{}) error {
	panic("not implemented yet")
}

func (g *govimplugin) RegisterCapability(ctxt context.Context, params *protocol.RegistrationParams) error {
	defer absorbShutdownErr()
	g.logGoplsClientf("RegisterCapability: %v", pretty.Sprint(params))
	return nil
}

func (g *govimplugin) UnregisterCapability(context.Context, *protocol.UnregistrationParams) error {
	panic("not implemented yet")
}

func (g *govimplugin) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	panic("not implemented yet")
}
func (g *govimplugin) Configuration(ctxt context.Context, params *protocol.ParamConfig) ([]interface{}, error) {
	defer absorbShutdownErr()
	g.logGoplsClientf("Configuration: %v", pretty.Sprint(params))

	// gopls now sends params.Items for each of the configured
	// workspaces. For now, we assume that the first item will be
	// for the section "gopls" and only configure that. We will
	// configure further workspaces when we add support for them.
	if len(params.Items) == 0 || params.Items[0].Section != "gopls" {
		return nil, fmt.Errorf("govim gopls client: expected at least one item, with the first section \"gopls\"")
	}
	res := make([]interface{}, len(params.Items))
	conf := make(map[string]interface{})
	conf[goplsConfigHoverKind] = "FullDocumentation"
	conf[goplsDisableDeepCompletion] = g.vimstate.config.CompletionDeepCompletionsDisable
	conf[goplsDisableFuzzyMatching] = g.vimstate.config.CompletionFuzzyMatchingDisable
	res[0] = conf

	g.logGoplsClientf("Configuration response: %v", pretty.Sprint(res))
	return res, nil
}

func (g *govimplugin) ApplyEdit(context.Context, *protocol.ApplyWorkspaceEditParams) (*protocol.ApplyWorkspaceEditResponse, error) {
	panic("not implemented yet")
}

func (g *govimplugin) Event(context.Context, *interface{}) error {
	panic("not implemented yet")
}

func (g *govimplugin) PublishDiagnostics(ctxt context.Context, params *protocol.PublishDiagnosticsParams) error {
	defer absorbShutdownErr()
	g.logGoplsClientf("PublishDiagnostics callback: %v", pretty.Sprint(params))
	g.diagnosticsLock.Lock()
	uri := span.URI(params.URI)
	curr, ok := g.diagnostics[uri]
	g.diagnostics[uri] = params
	g.diagnosticsLock.Unlock()
	if !ok {
		if len(params.Diagnostics) == 0 {
			return nil
		}
	} else if reflect.DeepEqual(curr, params) {
		// Whilst we await a solution https://github.com/golang/go/issues/32443
		// use reflect.DeepEqual to avoid hard-coding the comparison
		return nil
	}

	g.Schedule(func(govim.Govim) error {
		v := g.vimstate
		v.diagnosticsChanged = true
		if v.config.QuickfixAutoDiagnosticsDisable {
			return nil
		}
		if !v.quickfixIsDiagnostics {
			return nil
		}
		if v.userBusy {
			return nil
		}
		return v.redefineDiagnostics()
	})
	return nil
}

func absorbShutdownErr() {
	if r := recover(); r != nil && r != govim.ErrShuttingDown {
		panic(r)
	}
}

func (g *govimplugin) logGoplsClientf(format string, args ...interface{}) {
	if format[len(format)-1] != '\n' {
		format = format + "\n"
	}
	g.Logf("gopls client start =======================\n"+format+"gopls client end =======================\n", args...)
}
