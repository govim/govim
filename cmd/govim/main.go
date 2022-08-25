// Command govim is a Vim8 channel-based plugin, written in Go, to support the writing of Go code in Vim8
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/jsonrpc2"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/types"
	"github.com/govim/govim/cmd/govim/internal/vimconfig"
	"github.com/govim/govim/internal/plugin"
	"github.com/govim/govim/testsetup"
	"gopkg.in/tomb.v2"
)

const (
	PluginPrefix = "GOVIM"
)

// exposeTestAPI is a rather hacky but clean way of only exposing certain
// functions, commands and autocommands to Vim when run from a test
var exposeTestAPI = os.Getenv(testsetup.EnvLoadTestAPI) == "true"

func mainerr() error {
	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return flagErr(err.Error())
	}

	if *fParent != "" {
		return runAsChild()
	}

	var goplsPath string
	if os.Getenv(string(config.EnvVarUseGoplsFromPath)) == "true" {
		gopls, err := exec.LookPath("gopls")
		if err != nil {
			return fmt.Errorf("failed to find gopls in PATH: %v", err)
		}
		goplsPath = gopls
	} else {
		if flagSet.NArg() == 0 {
			return fmt.Errorf("missing single argument path to gopls")
		}
		goplsPath = flagSet.Arg(0)
	}

	if sock := os.Getenv(testsetup.EnvTestSocket); sock != "" {
		ln, err := net.Listen("tcp", sock)
		if err != nil {
			return fmt.Errorf("failed to listen on %v: %v", sock, err)
		}
		for {
			conn, err := ln.Accept()
			if err != nil {
				return fmt.Errorf("failed to accept connection on %v: %v", sock, err)
			}

			go func() {
				if err := launch(goplsPath, conn, conn); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}()
		}
	} else {
		return launch(goplsPath, os.Stdin, os.Stdout)
	}
}

func launch(goplspath string, in io.ReadCloser, out io.WriteCloser) error {
	defer out.Close()

	d, err := newplugin(goplspath, nil, nil, nil)
	if err != nil {
		return err
	}

	var logFile *os.File
	var writers []io.Writer

	if d.logging["on"] {
		logFile, err = d.createLogFile("govim")
		if err != nil {
			return err
		}
		defer logFile.Close()
		writers = append(writers, logFile)
		if os.Getenv(testsetup.EnvTestSocket) != "" {
			fmt.Fprintf(os.Stderr, "New connection will log to %v\n", logFile.Name())
		}
	}

	if *fTail {
		writers = append(writers, os.Stdout)
	}
	log := io.MultiWriter(writers...)

	g, err := govim.NewGovim(d, in, out, log, logFile, &d.tomb)
	if err != nil {
		return fmt.Errorf("failed to create govim instance: %v", err)
	}

	d.tomb.Go(g.Run)
	return d.tomb.Wait()
}

func (g *govimplugin) createLogFile(prefix string) (*os.File, error) {
	var tf *os.File
	var err error
	logfiletmpl := getEnvVal(g.goplsEnv, config.EnvLogfileTmpl, "%v_%v_%v")
	logfiletmpl += ".log"
	logfiletmpl = strings.Replace(logfiletmpl, "%v", prefix, 1)
	logfiletmpl = strings.Replace(logfiletmpl, "%v", time.Now().Format("20060102_1504_05"), 1)
	if strings.Contains(logfiletmpl, "%v") {
		logfiletmpl = strings.Replace(logfiletmpl, "%v", "*", 1)
		tf, err = ioutil.TempFile(g.tmpDir, logfiletmpl)
	} else {
		// append to existing file
		tf, err = os.OpenFile(filepath.Join(g.tmpDir, logfiletmpl), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	}
	if err != nil {
		err = fmt.Errorf("failed to create log file: %v", err)
	}
	return tf, err
}

type govimplugin struct {
	plugin.Driver
	vimstate *vimstate

	// errCh is the channel passed from govim on Init
	errCh chan error

	// logging is used to determine what to log. It can be used both as a
	// way to limit logging as well as a way to extend it. The value is
	// inherited from env var GOVIM_LOG.
	logging map[string]bool

	// tmpDir is the temp directory within which log files will be created
	tmpDir string

	// goplsEnv is the environment with which to start gopls. This is
	// set in os/exec.Command.Env
	goplsEnv []string

	goplspath   string
	gopls       *os.Process
	goplsConn   jsonrpc2.Conn
	goplsCancel context.CancelFunc
	goplsStdin  io.WriteCloser
	server      protocol.Server

	isGui bool

	tomb tomb.Tomb

	modWatcher *modWatcher

	// diagnosticsChangedLock protects access to rawDiagnostics,
	// diagnosticsChanged, diagnosticsChangedQuickfix,
	// diagnosticsChangedSigns and diagnosticsChangedHighlights
	diagnosticsChangedLock sync.Mutex

	// rawDiagnostics holds the current raw (LSP) diagnostics by URI
	rawDiagnostics map[span.URI]*protocol.PublishDiagnosticsParams

	// diagnosticsChanged indicates that the new diagnostics are available
	diagnosticsChanged bool

	// lastDiagnosticsQuickfix records the last diagnostics that were used
	// when updating the quickfix window
	lastDiagnosticsQuickfix *[]types.Diagnostic

	// lastDiagnosticsSigns records the last diagnostics that were used when
	// updating signs
	lastDiagnosticsSigns *[]types.Diagnostic

	// lastDiagnosticsHighlights records the last diagnostics that were used
	// when updating highlights
	lastDiagnosticsHighlights *[]types.Diagnostic

	// diagnosticsCache isn't inteded to be used directly since it might
	// contain old data. Call diagnostics() to get the latest instead.
	diagnosticsCache *[]types.Diagnostic

	// cancelDocHighlight is the function to cancel the ongoing LSP documentHighlight call. It must
	// be called before assigning a new value (or nil) to it. It is nil when there is no ongoing
	// call.
	cancelDocHighlight     context.CancelFunc
	cancelDocHighlightLock sync.Mutex

	// cancelSemTokRangeBuf is the function to cancel the ongoing LSP semanticTokens/range call,
	// per window ID. It must be called before assigning a new value (or nil) to it. It is
	// nil when there is no ongoing call.
	cancelSemTokRangeBuf  map[int]context.CancelFunc
	cancelSemTokRangeLock sync.Mutex

	// applyEditsCh is used to pass incoming edit requests (ApplyEdit) to the main thread.
	// Incoming ApplyEdit calls will use this channel if set (not nil) instead of schedule
	// edits directly. It is used to allow process edits during a blocking call on the vim
	// thread. Setting and unsetting this channel is protected by applyEditsLock.
	applyEditsCh   chan applyEditCall
	applyEditsLock sync.Mutex

	bufferUpdates chan *bufferUpdate

	// inShutdown is closed when govim is told to Shutdown
	inShutdown chan struct{}

	// socketDir is the temporary directory within which the parent-child
	// socket file will be created
	socketDir string

	// socketListener is the parent-child listener
	socketListener net.Listener

	// parentCallArgs represents the command that should be run to create a
	// "child" instance of govim to communicate with its "parent" (the instance
	// which responded to this function call)
	parentCallArgs []string
}

func newplugin(goplspath string, goplsEnv []string, defaults, user *config.Config) (*govimplugin, error) {
	if goplsEnv == nil {
		goplsEnv = os.Environ()
	}
	logging := make(map[string]bool)
	for _, v := range strings.Split(getEnvVal(goplsEnv, config.EnvLog, "on"), ",") {
		logging[v] = true
	}
	tmpDir := getEnvVal(goplsEnv, "TMPDIR", os.TempDir())
	if defaults == nil {
		defaults = &config.Config{
			FormatOnSave:                      vimconfig.FormatOnSaveVal(config.FormatOnSaveGoImportsGoFmt),
			QuickfixAutoDiagnostics:           vimconfig.BoolVal(true),
			QuickfixSigns:                     vimconfig.BoolVal(true),
			Staticcheck:                       vimconfig.BoolVal(false),
			HighlightDiagnostics:              vimconfig.BoolVal(true),
			HighlightReferences:               vimconfig.BoolVal(true),
			HoverDiagnostics:                  vimconfig.BoolVal(true),
			TempModfile:                       vimconfig.BoolVal(false),
			ExperimentalAutoreadLoadedBuffers: vimconfig.BoolVal(false),
			SymbolMatcher:                     vimconfig.SymbolMatcherVal(config.SymbolMatcherFuzzy),
			SymbolStyle:                       vimconfig.SymbolStyleVal(config.SymbolStyleFull),
			OpenLastProgressWith:              vimconfig.StringVal("below 10split"),
		}
	}
	// Overlay the initial user values on the defaults
	if user != nil {
		defaults.Apply(user)
	}
	d := plugin.NewDriver(PluginPrefix)
	var emptyDiags []types.Diagnostic
	res := &govimplugin{
		logging:              logging,
		tmpDir:               tmpDir,
		rawDiagnostics:       make(map[span.URI]*protocol.PublishDiagnosticsParams),
		cancelSemTokRangeBuf: make(map[int]context.CancelFunc),
		goplsEnv:             goplsEnv,
		goplspath:            goplspath,
		Driver:               d,
		inShutdown:           make(chan struct{}),
		diagnosticsCache:     &emptyDiags,
		vimstate: &vimstate{
			Driver:               d,
			buffers:              make(map[int]*types.Buffer),
			defaultConfig:        *defaults,
			config:               *defaults,
			suggestedFixesPopups: make(map[int][]suggestedFix),
			progressPopups:       make(map[protocol.ProgressToken]*types.ProgressPopup),
			semanticTokens: struct {
				lock  sync.Mutex
				types map[uint32]string
				mods  map[uint32]string
			}{
				types: make(map[uint32]string),
				mods:  make(map[uint32]string),
			},
			placedSemanticTokens: make(map[int]struct {
				bufnr int
				from  int
				to    int
			}),
		},
	}
	res.vimstate.govimplugin = res
	return res, nil
}

func (g *govimplugin) Init(gg govim.Govim, errCh chan error) error {
	// Start the parent server first, because it establishes the command []string
	// that forms the response to GOVIMParentCommand()
	if err := g.startParentServer(); err != nil {
		return err
	}

	g.errCh = errCh
	g.Driver.Govim = gg
	g.vimstate.Driver.Govim = gg.Scheduled()
	g.vimstate.workingDirectory = g.ParseString(g.ChannelCall("getcwd", -1))
	g.DefineFunction(string(config.FunctionBalloonExpr), []string{}, g.vimstate.balloonExpr)
	g.DefineAutoCommand("", govim.Events{govim.EventBufUnload}, govim.Patterns{"*.go", "go.mod", "go.sum"}, false, g.vimstate.bufUnload, "eval(expand('<abuf>'))")
	g.DefineAutoCommand("", govim.Events{govim.EventBufRead, govim.EventBufNewFile}, govim.Patterns{"*.go", "go.mod", "go.sum"}, false, g.vimstate.bufReadPost, exprAutocmdCurrBufInfo)
	g.DefineAutoCommand("", govim.Events{govim.EventBufWritePre}, govim.Patterns{"*.go", "go.mod", "go.sum"}, false, g.vimstate.formatCurrentBuffer, "eval(expand('<abuf>'))")
	g.DefineAutoCommand("", govim.Events{govim.EventBufWritePost}, govim.Patterns{"*.go", "go.mod", "go.sum"}, false, g.vimstate.bufWritePost, "eval(expand('<abuf>'))")
	g.DefineAutoCommand("", govim.Events{govim.EventQuickFixCmdPre}, govim.Patterns{"*vimgrep*"}, false, g.vimstate.bufQuickFixCmdPre)
	g.DefineAutoCommand("", govim.Events{govim.EventQuickFixCmdPost}, govim.Patterns{"*vimgrep*"}, false, g.vimstate.bufQuickFixCmdPost)
	g.DefineFunction(string(config.FunctionComplete), []string{"findarg", "base"}, g.vimstate.complete)
	g.DefineCommand(string(config.CommandGoToDef), g.vimstate.gotoDef, govim.NArgsZeroOrOne)
	g.DefineCommand(string(config.CommandGoToTypeDef), g.vimstate.gotoTypeDef, govim.NArgsZeroOrOne)
	g.DefineCommand(string(config.CommandSuggestedFixes), g.vimstate.suggestFixes, govim.NArgsZeroOrOne)
	g.DefineCommand(string(config.CommandGoToPrevDef), g.vimstate.gotoPrevDef, govim.NArgsZeroOrOne, govim.CountN(1))
	g.DefineFunction(string(config.FunctionHover), []string{}, g.vimstate.hover)
	g.DefineAutoCommand("", govim.Events{govim.EventBufDelete}, govim.Patterns{"*.go", "go.mod", "go.sum"}, false, g.vimstate.bufDelete, "eval(expand('<abuf>'))")
	g.DefineAutoCommand("", govim.Events{govim.EventBufWipeout}, govim.Patterns{"*.go", "go.mod", "go.sum"}, false, g.vimstate.bufWipeout, "eval(expand('<abuf>'))")
	g.DefineCommand(string(config.CommandGoFmt), g.vimstate.gofmtCurrentBufferRange)
	g.DefineCommand(string(config.CommandGoImports), g.vimstate.goimportsCurrentBufferRange)
	g.DefineCommand(string(config.CommandQuickfixDiagnostics), g.vimstate.quickfixDiagnostics)
	g.DefineFunction(string(config.FunctionBufChanged), []string{"bufnr", "start", "end", "added", "changes"}, g.vimstate.bufChanged)
	g.DefineFunction(string(config.FunctionSetConfig), []string{"config"}, g.vimstate.setConfig)
	g.ChannelExf(`call govim#config#Set("%vFunc", function("%v%v"))`, config.InternalFunctionPrefix, PluginPrefix, config.FunctionSetConfig)
	g.DefineFunction(string(config.FunctionSetUserBusy), []string{"isBusy", "cursorPos"}, g.vimstate.setUserBusy)
	g.DefineFunction(string(config.FunctionPopupSelection), []string{"id", "selected"}, g.vimstate.popupSelection)
	g.DefineFunction(string(config.FunctionVisibleLines), []string{"buffers"}, g.vimstate.visibleLines)
	g.DefineCommand(string(config.CommandReferences), g.vimstate.references)
	g.DefineCommand(string(config.CommandImplements), g.vimstate.implements)
	g.DefineCommand(string(config.CommandRename), g.vimstate.rename, govim.NArgsZeroOrOne)
	g.DefineCommand(string(config.CommandStringFn), g.vimstate.stringfns, govim.RangeLine, govim.CompleteCustomList(PluginPrefix+config.FunctionStringFnComplete), govim.NArgsOneOrMore)
	g.DefineFunction(string(config.FunctionStringFnComplete), []string{"ArgLead", "CmdLine", "CursorPos"}, g.vimstate.stringfncomplete)
	g.DefineCommand(string(config.CommandHighlightReferences), g.vimstate.highlightReferences)
	g.DefineCommand(string(config.CommandClearReferencesHighlights), g.vimstate.clearReferencesHighlights)
	g.DefineAutoCommand("", govim.Events{govim.EventCompleteDone}, govim.Patterns{"*.go", "go.mod", "go.sum"}, false, g.vimstate.completeDone, "eval(expand('<abuf>'))", "v:completed_item")
	g.DefineFunction(string(config.FunctionParentCommand), []string{}, g.vimstate.parentCommand)
	g.DefineCommand(string(config.CommandExperimentalSignatureHelp), g.vimstate.signatureHelp)
	g.DefineCommand(string(config.CommandFillStruct), g.vimstate.fillStruct)
	g.DefineCommand(string(config.CommandGCDetails), g.vimstate.toggleGCDetails)
	g.DefineCommand(string(config.CommandGoTest), g.vimstate.runGoTest, govim.RangeLine)
	g.DefineFunction(string(config.FunctionProgressClosed), []string{"id", "selected"}, g.vimstate.progressClosed)
	g.DefineCommand(string(config.CommandLastProgress), g.vimstate.openLastProgress)
	g.defineHighlights()
	if err := g.vimstate.signDefine(); err != nil {
		return fmt.Errorf("failed to define signs: %v", err)
	}
	if err := g.vimstate.textpropDefine(); err != nil {
		return fmt.Errorf("failed to defined text property types: %v", err)
	}
	g.DefineFunction(string(config.FunctionMotion), []string{"direction", "target"}, g.vimstate.motion)

	g.startProcessBufferUpdates()

	g.InitTestAPI()

	g.isGui = g.ParseInt(g.ChannelExpr(`has("gui_running")`)) == 1

	if err := g.startGopls(); err != nil {
		return err
	}

	return nil
}

func goModSpecPath(wd string) (string, error) {
	// Workspace mode is prioritised as per https://go.dev/issue/50955 and https://go.dev/ref/mod#workspaces
	// and "go env GOWORK" tells us if we are in workspace mode. If we aren't, we assume module mode.
	cmd := exec.Command("go", "env", "-json", "GOWORK", "GOMOD")
	cmd.Dir = wd

	// From https://github.com/golang/tools/blob/fe37c9e135b934191089b245ac29325091462508/internal/gocommand/invoke.go#L208:
	//
	// On darwin the cwd gets resolved to the real path, which breaks anything that
	// expects the working directory to keep the original path, including the
	// go command when dealing with modules.
	// The Go stdlib has a special feature where if the cwd and the PWD are the
	// same node then it trusts the PWD, so by setting it in the env for the child
	// process we fix up all the paths returned by the go command.
	cmd.Env = append(os.Environ(), "PWD="+wd)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute %q in %v: %v\n%s", cmd.Args, wd, err, out)
	}

	envs := struct {
		GOMOD  string `json:"GOMOD"`
		GOWORK string `json:"GOWORK"`
	}{}
	if err := json.Unmarshal(out, &envs); err != nil {
		return "", fmt.Errorf("failed to unmarshal output from 'go env -json': %v", err)
	}

	if envs.GOWORK != "" {
		return envs.GOWORK, nil
	}
	return envs.GOMOD, nil
}

// Shutdown implements the govim.Plugin Shutdown method.
//
// TODO: because we do not trigger a shutdown from Vim (we simply close the
// channel and then exit Vim without waiting for govim to complete its
// shutdown) we are not guaranteed that this method actually runs when Vim
// exits. Hence any temporary files/directories that we (or another process)
// have created might not actually get removed. We should trigger a proper
// shutdown sequence from Vim that ultimately calls this method before closing
// the channel; this is a similar protocol to that used in LSP. For more details
// see: github.com/govim/govim/issues/842
func (g *govimplugin) Shutdown() error {
	close(g.inShutdown)
	close(g.bufferUpdates)

	// Tidy up the parent-child socket listener
	if err := g.socketListener.Close(); err != nil {
		return fmt.Errorf("failed to close the parent-child socket listener: %v", err)
	}
	os.RemoveAll(g.socketDir) // see note above

	// TODO: remove this workaround for golang.org/issue/45476. We should not
	// have to set a deadline for our call to Shutdown.  500ms is the longest a
	// user should be expected to wait before "forcing" the shutdown sequence.
	// Because of golang.org/issue/45476, the shorter we make this timeout the
	// greater the likelihood that gopls will leave $TMPDIR artefacts lying
	// around, and not have properly tidied up after itself.
	ctxt, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := g.server.Shutdown(ctxt); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("failed to call gopls Shutdown: %v", err)
	}
	// As the initiator of the connection to gopls, complete shutdown by closing
	// stdin to the process we started.
	if err := g.goplsStdin.Close(); err != nil {
		return fmt.Errorf("failed to close gopls stdin: %v", err)
	}

	// Shutdown the filewatcher
	if g.modWatcher != nil {
		if err := g.modWatcher.close(); err != nil {
			return fmt.Errorf("failed to close file watcher: %v", err)
		}
	}
	return nil
}

func (g *govimplugin) defineHighlights() {
	warnColor := 166    // Orange
	diagSrcColor := 245 // Grey #8a8a8a
	if vimColors, err := strconv.Atoi(g.ParseString(g.ChannelExpr(`&t_Co`))); err != nil || vimColors < 256 {
		warnColor = 3    // Yellow, fallback when the terminal doesn't support at least 256 colors
		diagSrcColor = 7 // Silver
	}
	g.vimstate.BatchStart()
	for _, hi := range []string{
		fmt.Sprintf("highlight default %s term=underline cterm=underline gui=undercurl ctermfg=1 guisp=Red", config.HighlightErr),
		fmt.Sprintf("highlight default %s term=underline cterm=underline gui=undercurl ctermfg=%d guisp=Orange", config.HighlightWarn, warnColor),
		fmt.Sprintf("highlight default %s term=underline cterm=underline gui=undercurl ctermfg=6 guisp=Cyan", config.HighlightInfo),
		fmt.Sprintf("highlight default link %s %s", config.HighlightHint, config.HighlightInfo),

		fmt.Sprintf("highlight default link %s ErrorMsg", config.HighlightSignErr),
		fmt.Sprintf("highlight default %s ctermfg=15 ctermbg=%d guisp=Orange guifg=Orange", config.HighlightSignWarn, warnColor),
		fmt.Sprintf("highlight default %s ctermfg=15 ctermbg=6 guisp=Cyan guifg=Cyan", config.HighlightSignInfo),
		fmt.Sprintf("highlight default link %s %s", config.HighlightSignHint, config.HighlightSignInfo),

		fmt.Sprintf("highlight default %s cterm=bold gui=bold ctermfg=1", config.HighlightHoverErr),
		fmt.Sprintf("highlight default %s cterm=bold gui=bold ctermfg=%d", config.HighlightHoverWarn, warnColor),
		fmt.Sprintf("highlight default %s cterm=bold gui=bold ctermfg=6", config.HighlightHoverInfo),
		fmt.Sprintf("highlight default link %s %s", config.HighlightHoverHint, config.HighlightHoverInfo),

		fmt.Sprintf("highlight default %s cterm=none gui=italic ctermfg=%d guifg=#8a8a8a", config.HighlightHoverDiagSrc, diagSrcColor),

		fmt.Sprintf("highlight default %s term=reverse cterm=reverse gui=reverse", config.HighlightReferences),

		fmt.Sprintf("highlight default link %s PMenu", config.HighlightSignature),
		fmt.Sprintf("highlight default %s term=bold cterm=bold gui=bold", config.HighlightSignatureParam),

		fmt.Sprintf("highlight default %s ctermfg=2 guifg=Green", config.HighlightGoTestPass),
		fmt.Sprintf("highlight default %s ctermfg=1 guifg=Red ", config.HighlightGoTestFail),

		fmt.Sprintf("highlight default link %s Operator", config.HighlightSemTokNamespace),
		fmt.Sprintf("highlight default link %s Type", config.HighlightSemTokType),
		fmt.Sprintf("highlight default link %s Error", config.HighlightSemTokClass),
		fmt.Sprintf("highlight default link %s Error", config.HighlightSemTokEnum),
		fmt.Sprintf("highlight default link %s Error", config.HighlightSemTokInterface),
		fmt.Sprintf("highlight default link %s Error", config.HighlightSemTokStruct),
		fmt.Sprintf("highlight default link %s Identifier", config.HighlightSemTokTypeParameter),
		fmt.Sprintf("highlight default link %s Identifier", config.HighlightSemTokParameter),
		fmt.Sprintf("highlight default link %s Normal", config.HighlightSemTokVariable),
		fmt.Sprintf("highlight default link %s Error", config.HighlightSemTokProperty),
		fmt.Sprintf("highlight default link %s Error", config.HighlightSemTokEnumMember),
		fmt.Sprintf("highlight default link %s Error", config.HighlightSemTokEvent),
		fmt.Sprintf("highlight default link %s Function", config.HighlightSemTokFunction),
		fmt.Sprintf("highlight default link %s Function", config.HighlightSemTokMethod),
		fmt.Sprintf("highlight default link %s Error", config.HighlightSemTokMacro),
		fmt.Sprintf("highlight default link %s Keyword", config.HighlightSemTokKeyword),
		fmt.Sprintf("highlight default link %s Error", config.HighlightSemTokModifier),
		fmt.Sprintf("highlight default link %s Comment", config.HighlightSemTokComment),
		fmt.Sprintf("highlight default link %s String", config.HighlightSemTokString),
		fmt.Sprintf("highlight default link %s Number", config.HighlightSemTokNumber),
		fmt.Sprintf("highlight default link %s Error", config.HighlightSemTokRegexp),
		fmt.Sprintf("highlight default link %s Operator", config.HighlightSemTokOperator),
	} {
		g.vimstate.BatchChannelCall("execute", hi)
	}
	g.vimstate.MustBatchEnd()
}

func getEnvVal(env []string, varname string, vdefault string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], varname+"=") {
			return strings.TrimPrefix(env[i], varname+"=")
		}
	}
	return vdefault
}
