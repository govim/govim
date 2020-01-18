package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/kr/pretty"
)

const (
	goplsConfigNoDocsOnHover  = "noDocsOnHover"
	goplsConfigHoverKind      = "hoverKind"
	goplsDeepCompletion       = "deepCompletion"
	goplsCompletionMatcher    = "matcher"
	goplsStaticcheck          = "staticcheck"
	goplsCompleteUnimported   = "completeUnimported"
	goplsGoImportsLocalPrefix = "local"
	goplsCompletionBudget     = "completionBudget"
)

var _ protocol.Client = (*govimplugin)(nil)

func (g *govimplugin) ShowMessage(ctxt context.Context, params *protocol.ShowMessageParams) error {
	defer absorbShutdownErr()
	g.logGoplsClientf("ShowMessage callback: %v", params.Message)

	var hl string
	switch params.Type {
	case protocol.Error:
		hl = "ErrorMsg"
	case protocol.Warning:
		hl = "WarningMsg"
	default:
		return nil
	}

	g.Schedule(func(govim.Govim) error {
		opts := make(map[string]interface{})
		opts["mousemoved"] = "any"
		opts["moved"] = "any"
		opts["padding"] = []int{0, 1, 0, 1}
		opts["wrap"] = true
		opts["border"] = []int{}
		opts["highlight"] = hl
		opts["line"] = 1
		opts["close"] = "click"

		g.ChannelCall("popup_create", strings.Split(params.Message, "\n"), opts)
		return nil
	})
	return nil
}

func (g *govimplugin) ShowMessageRequest(context.Context, *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	defer absorbShutdownErr()
	panic("not implemented yet")
}

func (g *govimplugin) LogMessage(ctxt context.Context, params *protocol.LogMessageParams) error {
	defer absorbShutdownErr()
	g.logGoplsClientf("LogMessage callback: %v", pretty.Sprint(params))
	return nil
}

func (g *govimplugin) Telemetry(context.Context, interface{}) error {
	defer absorbShutdownErr()
	panic("not implemented yet")
}

func (g *govimplugin) RegisterCapability(ctxt context.Context, params *protocol.RegistrationParams) error {
	defer absorbShutdownErr()
	g.logGoplsClientf("RegisterCapability: %v", pretty.Sprint(params))
	return nil
}

func (g *govimplugin) UnregisterCapability(context.Context, *protocol.UnregistrationParams) error {
	defer absorbShutdownErr()
	panic("not implemented yet")
}

func (g *govimplugin) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	defer absorbShutdownErr()
	panic("not implemented yet")
}

func (g *govimplugin) Configuration(ctxt context.Context, params *protocol.ParamConfiguration) ([]interface{}, error) {
	defer absorbShutdownErr()

	// TODO this is a rather fragile workaround for https://github.com/golang/go/issues/35817
	// It's fragile because we are relying on gopls not handling any requests until the response
	// to Configuration is received and processed. In practice this appears to currently be
	// the case but there is no guarantee of this going forward. Rather we hope that a fix
	// for https://github.com/golang/go/issues/35817 lands sooner rather than later at whic
	// point this workaround can go.
	//
	// We also use a lock here because, despite it appearing that will only be a single
	// Configuration call and that if there were more they would be serial, we can't rely on
	// this.
	defer func() {
		g.initalConfigurationCalledLock.Lock()
		defer g.initalConfigurationCalledLock.Unlock()
		select {
		case <-g.initalConfigurationCalled:
		default:
			close(g.initalConfigurationCalled)
		}
	}()

	g.logGoplsClientf("Configuration: %v", pretty.Sprint(params))

	g.vimstate.configLock.Lock()
	config := g.vimstate.config
	defer g.vimstate.configLock.Unlock()

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
	if config.CompletionDeepCompletions != nil {
		conf[goplsDeepCompletion] = *config.CompletionDeepCompletions
	}
	if config.CompletionMatcher != nil {
		conf[goplsCompletionMatcher] = *config.CompletionMatcher
	}
	if config.Staticcheck != nil {
		conf[goplsStaticcheck] = *config.Staticcheck
	}
	if config.CompleteUnimported != nil {
		conf[goplsCompleteUnimported] = *config.CompleteUnimported
	}
	if g.vimstate.config.GoImportsLocalPrefix != nil {
		conf[goplsGoImportsLocalPrefix] = *config.GoImportsLocalPrefix
	}
	if g.vimstate.config.CompletionBudget != nil {
		conf[goplsCompletionBudget] = *config.CompletionBudget
	}
	res[0] = conf

	g.logGoplsClientf("Configuration response: %v", pretty.Sprint(res))
	return res, nil
}

func (g *govimplugin) ApplyEdit(context.Context, *protocol.ApplyWorkspaceEditParams) (*protocol.ApplyWorkspaceEditResponse, error) {
	defer absorbShutdownErr()
	panic("not implemented yet")
}

func (g *govimplugin) Event(context.Context, *interface{}) error {
	defer absorbShutdownErr()
	panic("not implemented yet")
}

func (g *govimplugin) PublishDiagnostics(ctxt context.Context, params *protocol.PublishDiagnosticsParams) error {
	defer absorbShutdownErr()
	g.logGoplsClientf("PublishDiagnostics callback: %v", pretty.Sprint(params))
	g.diagnosticsChangedLock.Lock()
	uri := span.URI(params.URI)
	curr, ok := g.rawDiagnostics[uri]
	// TODO: add some temp logging for https://github.com/golang/go/issues/36601
	// Note this only captures situations where a file's version is increasing.
	// Because it's possible to receive new diagnostics for a file without its
	// version increasing. And in that case it's impossible to know if the
	// diagnostics we receive are old or not.
	if ok && curr.Version > params.Version {
		g.Logf("** Received non-new diagnostics for %v; ignoring. Currently have: \n%v\nGot: \n%v", params.URI, tabIndent(pretty.Sprint(curr)), tabIndent(pretty.Sprint(params)))
	}
	g.rawDiagnostics[uri] = params
	g.diagnosticsChanged = true
	g.diagnosticsChangedQuickfix = true
	g.diagnosticsChangedSigns = true
	g.diagnosticsChangedLock.Unlock()
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
		if v.userBusy {
			return nil
		}
		return v.handleDiagnosticsChanged()
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

func tabIndent(s string) string {
	return "\t" + strings.ReplaceAll(s, "\n", "\n\t")
}
