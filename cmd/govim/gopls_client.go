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
	goplsConfigNoDocsOnHover     = "noDocsOnHover"
	goplsConfigHoverKind         = "hoverKind"
	goplsDeepCompletion          = "deepCompletion"
	goplsFuzzyMatching           = "fuzzyMatching"
	goplsStaticcheck             = "staticcheck"
	goplsCaseSensitiveCompletion = "caseSensitiveCompletion"
	goplsCompleteUnimported      = "completeUnimported"
	goplsGoImportsLocalPrefix    = "local"
)

var _ protocol.Client = (*govimplugin)(nil)

func (g *govimplugin) ShowMessage(ctxt context.Context, params *protocol.ShowMessageParams) error {
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
		opts["wrap"] = false
		opts["border"] = []int{}
		opts["highlight"] = hl
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
	if g.vimstate.config.CompletionDeepCompletions != nil {
		conf[goplsDeepCompletion] = *g.vimstate.config.CompletionDeepCompletions
	}
	if g.vimstate.config.CompletionFuzzyMatching != nil {
		conf[goplsFuzzyMatching] = *g.vimstate.config.CompletionFuzzyMatching
	}
	if g.vimstate.config.Staticcheck != nil {
		conf[goplsStaticcheck] = *g.vimstate.config.Staticcheck
	}
	if g.vimstate.config.CompletionCaseSensitive != nil {
		conf[goplsCaseSensitiveCompletion] = *g.vimstate.config.CompletionCaseSensitive
	}
	if g.vimstate.config.CompleteUnimported != nil {
		conf[goplsCompleteUnimported] = *g.vimstate.config.CompleteUnimported
	}
	if g.vimstate.config.GoImportsLocalPrefix != nil {
		conf[goplsGoImportsLocalPrefix] = *g.vimstate.config.GoImportsLocalPrefix
	}
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
	g.rawDiagnosticsLock.Lock()
	uri := span.URI(params.URI)
	curr, ok := g.rawDiagnostics[uri]
	g.rawDiagnostics[uri] = params
	g.rawDiagnosticsLock.Unlock()
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
		if v.config.QuickfixAutoDiagnostics != nil && !*v.config.QuickfixAutoDiagnostics {
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
