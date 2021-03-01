package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"unicode"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/command"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/types"
	"github.com/kr/pretty"
)

const (
	goplsConfigNoDocsOnHover         = "noDocsOnHover"
	goplsConfigHoverKind             = "hoverKind"
	goplsDeepCompletion              = "deepCompletion"
	goplsCompletionMatcher           = "matcher"
	goplsStaticcheck                 = "staticcheck"
	goplsCompleteUnimported          = "completeUnimported"
	goplsGoImportsLocalPrefix        = "local"
	goplsCompletionBudget            = "completionBudget"
	goplsTempModfile                 = "tempModfile"
	goplsVerboseOutput               = "verboseOutput"
	goplsEnv                         = "env"
	goplsAnalyses                    = "analyses"
	goplsCodeLenses                  = "codelenses"
	goplsSymbolMatcher               = "symbolMatcher"
	goplsSymbolStyle                 = "symbolStyle"
	goplsGofumpt                     = "gofumpt"
	goplsExperimentalWorkspaceModule = "experimentalWorkspaceModule"
)

var _ protocol.Client = (*govimplugin)(nil)

func (g *govimplugin) ShowMessage(ctxt context.Context, params *protocol.ShowMessageParams) error {
	defer absorbShutdownErr()
	g.logGoplsClientf("ShowMessage callback: %v", pretty.Sprint(params))

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
	panic("ShowMessageRequest not implemented yet")
}

func (g *govimplugin) LogMessage(ctxt context.Context, params *protocol.LogMessageParams) error {
	defer absorbShutdownErr()
	g.logGoplsClientf("LogMessage callback: %v", pretty.Sprint(params))
	return nil
}

func (g *govimplugin) Telemetry(context.Context, interface{}) error {
	defer absorbShutdownErr()
	panic("Telemetry not implemented yet")
}

func (g *govimplugin) RegisterCapability(ctxt context.Context, params *protocol.RegistrationParams) error {
	defer absorbShutdownErr()
	for _, r := range params.Registrations {
		switch r.Method {
		case "workspace/didChangeConfiguration":
			// For now ignore per #949
		case "workspace/didChangeWorkspaceFolders":
			// For now ignore per #172
		case "workspace/didChangeWatchedFiles":
			// For now ignore per #950
		default:
			panic(fmt.Errorf("RegisterCapability called with unknown method: %v", pretty.Sprint(params)))
		}
	}
	g.logGoplsClientf("RegisterCapability: %v", pretty.Sprint(params))
	return nil
}

func (g *govimplugin) UnregisterCapability(ctxt context.Context, params *protocol.UnregistrationParams) error {
	defer absorbShutdownErr()
	for _, u := range params.Unregisterations {
		switch u.Method {
		case "workspace/didChangeConfiguration":
			// For now ignore per #949
		case "workspace/didChangeWorkspaceFolders":
			// For now ignore per #172
		case "workspace/didChangeWatchedFiles":
		// 	// For now ignore per #950
		default:
			panic(fmt.Errorf("UnregisterCapability called with unknown method: %v", pretty.Sprint(params)))
		}
	}
	g.logGoplsClientf("UnregisterCapability: %v", pretty.Sprint(params))
	return nil
}

func (g *govimplugin) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	defer absorbShutdownErr()
	panic("WorkspaceFolders not implemented yet")
}

func (g *govimplugin) Configuration(ctxt context.Context, params *protocol.ParamConfiguration) ([]interface{}, error) {
	defer absorbShutdownErr()

	g.logGoplsClientf("Configuration: %v", pretty.Sprint(params))

	g.vimstate.configLock.Lock()
	conf := g.vimstate.config
	defer g.vimstate.configLock.Unlock()

	// gopls now sends params.Items for each of the configured
	// workspaces. For now, we assume that the first item will be
	// for the section "gopls" and only configure that. We will
	// configure further workspaces when we add support for them.
	if len(params.Items) == 0 || params.Items[0].Section != "gopls" {
		return nil, fmt.Errorf("govim gopls client: expected at least one item, with the first section \"gopls\"")
	}
	res := make([]interface{}, len(params.Items))
	goplsConfig := make(map[string]interface{})
	goplsConfig[goplsConfigHoverKind] = "FullDocumentation"
	if conf.CompletionDeepCompletions != nil {
		goplsConfig[goplsDeepCompletion] = *conf.CompletionDeepCompletions
	}
	if conf.CompletionMatcher != nil {
		goplsConfig[goplsCompletionMatcher] = *conf.CompletionMatcher
	}
	if conf.Staticcheck != nil {
		goplsConfig[goplsStaticcheck] = *conf.Staticcheck
	}
	if conf.CompleteUnimported != nil {
		goplsConfig[goplsCompleteUnimported] = *conf.CompleteUnimported
	}
	if conf.GoImportsLocalPrefix != nil {
		goplsConfig[goplsGoImportsLocalPrefix] = *conf.GoImportsLocalPrefix
	}
	if conf.CompletionBudget != nil {
		goplsConfig[goplsCompletionBudget] = *conf.CompletionBudget
	}
	if conf.TempModfile != nil {
		goplsConfig[goplsTempModfile] = *conf.TempModfile
	}
	if conf.Gofumpt != nil {
		goplsConfig[goplsGofumpt] = *conf.Gofumpt
	}
	if conf.ExperimentalWorkspaceModule != nil {
		goplsConfig[goplsExperimentalWorkspaceModule] = *conf.ExperimentalWorkspaceModule
	}
	if os.Getenv(string(config.EnvVarGoplsVerbose)) == "true" {
		goplsConfig[goplsVerboseOutput] = true
	}
	if conf.Analyses != nil {
		goplsConfig[goplsAnalyses] = *conf.Analyses
	}
	goplsConfig[goplsCodeLenses] = map[string]bool{
		string(command.GCDetails): true, // gc_details
	}
	if conf.GoplsEnv != nil {
		// It is safe not to copy the map here because a new config setting from
		// Vim creates a new map.
		goplsConfig[goplsEnv] = *conf.GoplsEnv
	}
	res[0] = goplsConfig

	g.logGoplsClientf("Configuration response: %v", pretty.Sprint(res))
	return res, nil
}

func (g *govimplugin) ApplyEdit(ctxt context.Context, params *protocol.ApplyWorkspaceEditParams) (*protocol.ApplyWorkspaceEditResponse, error) {
	defer absorbShutdownErr()
	g.logGoplsClientf("ApplyEdit: %v", pretty.Sprint(params))

	var err error
	var res *protocol.ApplyWorkspaceEditResponse
	g.applyEditsLock.Lock()
	if g.applyEditsCh == nil {
		// ApplyEdit wasn't send by another blocking call so it's fine to just schedule the edits here
		g.applyEditsLock.Unlock()
		done := make(chan struct{})
		g.Schedule(func(govim.Govim) error {
			v := g.vimstate
			res, err = v.applyWorkspaceEdit(params)
			close(done)
			return nil
		})
		<-done
	} else {
		// There is an ongoing call that can apply edits on the vim thread. A Schedule here would deadlock so
		// pass the edits to the vim thread instead.
		e := applyEditCall{params: params, responseCh: make(chan applyEditResponse)}
		g.applyEditsCh <- e
		aer := <-e.responseCh
		g.applyEditsLock.Unlock()
		res = aer.res
		err = aer.err
	}

	g.logGoplsClientf("ApplyEdit response: %v", pretty.Sprint(res))
	return res, err
}

func (g *govimplugin) Event(context.Context, *interface{}) error {
	defer absorbShutdownErr()
	panic("Event not implemented yet")
}

func (g *govimplugin) PublishDiagnostics(ctxt context.Context, params *protocol.PublishDiagnosticsParams) error {
	defer absorbShutdownErr()
	g.logGoplsClientf("PublishDiagnostics callback: %v", pretty.Sprint(params))
	g.diagnosticsChangedLock.Lock()
	uri := span.URI(params.URI)
	curr, ok := g.rawDiagnostics[uri]
	g.rawDiagnostics[uri] = params
	g.diagnosticsChanged = true
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

func (g *govimplugin) Progress(ctxt context.Context, params *protocol.ProgressParams) error {
	defer absorbShutdownErr()
	g.logGoplsClientf("Progress callback: %v", pretty.Sprint(params))

	g.vimstate.configLock.Lock()
	if c := g.vimstate.config.ExperimentalProgressPopups; c == nil || !*c {
		g.vimstate.configLock.Unlock()
		return nil
	}
	g.vimstate.configLock.Unlock()

	var ok bool
	var raw map[string]interface{}
	if raw, ok = params.Value.(map[string]interface{}); !ok {
		return fmt.Errorf("unexpected value type: %T", params.Value)
	}
	var kind string
	if kind, ok = raw["kind"].(string); !ok { // required by LSP
		return fmt.Errorf("expected required field 'kind' in progress callback")
	}
	message, _ := raw["message"].(string) // optional
	message = strings.TrimRightFunc(message, unicode.IsSpace)

	var title string
	if title, ok = raw["title"].(string); !ok && kind == "begin" { // required for "begin"
		return fmt.Errorf("expected required field 'title' not set")
	}

	g.Schedule(func(govim.Govim) error {
		v := g.vimstate

		popup, ok := v.progressPopups[params.Token]
		if !ok {
			return nil
		}

		return v.handleProgress(popup, kind, title, message)
	})
	return nil
}

func (g *govimplugin) WorkDoneProgressCreate(ctxt context.Context, params *protocol.WorkDoneProgressCreateParams) error {
	defer absorbShutdownErr()
	g.logGoplsClientf("WorkDoneProgressCreate callback: %v", pretty.Sprint(params))

	g.vimstate.configLock.Lock()
	if c := g.vimstate.config.ExperimentalProgressPopups; c == nil || !*c {
		g.vimstate.configLock.Unlock()
		return nil
	}
	g.vimstate.configLock.Unlock()

	g.Schedule(func(govim.Govim) error {
		v := g.vimstate
		if _, ok := v.progressPopups[params.Token]; ok {
			return fmt.Errorf("WorkDoneProgressCreate received for an ongoing progress token")
		}
		v.progressPopups[params.Token] = &types.ProgressPopup{Initiator: types.WorkDoneProgressCreate}
		return nil
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
