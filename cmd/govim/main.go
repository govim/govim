// Command govim is a Vim8 channel-based plugin, written in Go, to support the writing of Go code in Vim8
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
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
	"github.com/govim/govim/cmd/govim/internal/util"
	"github.com/govim/govim/cmd/govim/internal/vimconfig"
	"github.com/govim/govim/internal/plugin"
	"github.com/govim/govim/testsetup"
	"gopkg.in/tomb.v2"
)

const (
	PluginPrefix = "GOVIM"
)

var (
	fTail = flag.Bool("tail", false, "whether to also log output to stdout")

	// gopls define InitializationOptions; they will make these well defined
	// constants at some point
	goplsInitOptIncrementalSync = "incrementalSync"

	// exposeTestAPI is a rather hacky but clean way of only exposing certain
	// functions, commands and autocommands to Vim when run from a test
	exposeTestAPI = os.Getenv(testsetup.EnvLoadTestAPI) == "true"
)

func main() {
	os.Exit(main1())
}

func main1() int {
	switch err := mainerr(); err {
	case nil:
		return 0
	case flag.ErrHelp:
		return 2
	default:
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
}

func mainerr() error {
	flag.Parse()

	var goplsPath string
	if os.Getenv(string(config.EnvVarUseGoplsFromPath)) == "true" {
		gopls, err := exec.LookPath("gopls")
		if err != nil {
			return fmt.Errorf("failed to find gopls in PATH: %v", err)
		}
		goplsPath = gopls
	} else {
		if flag.NArg() == 0 {
			return fmt.Errorf("missing single argument path to gopls")
		}
		goplsPath = flag.Arg(0)
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

	d := newplugin(goplspath, nil, nil, nil)

	tf, err := d.createLogFile("govim")
	if err != nil {
		return err
	}
	defer tf.Close()

	var log io.Writer = tf
	if *fTail {
		log = io.MultiWriter(tf, os.Stdout)
	}

	if os.Getenv(testsetup.EnvTestSocket) != "" {
		fmt.Fprintf(os.Stderr, "New connection will log to %v\n", tf.Name())
	}

	g, err := govim.NewGovim(d, in, out, log, &d.tomb)
	if err != nil {
		return fmt.Errorf("failed to create govim instance: %v", err)
	}

	d.tomb.Go(g.Run)
	return d.tomb.Wait()
}

func (g *govimplugin) createLogFile(prefix string) (*os.File, error) {
	var tf *os.File
	var err error
	logfiletmpl := getEnvVal(g.goplsEnv, testsetup.EnvLogfileTmpl)
	if logfiletmpl == "" {
		logfiletmpl = "%v_%v_%v"
	}
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

	// tmpDir is the temp directory within which log files will be created
	tmpDir string

	// goplsEnv is the environment with which to start gopls. This is
	// set in os/exec.Command.Env
	goplsEnv []string

	goplspath   string
	gopls       *os.Process
	goplsConn   *jsonrpc2.Conn
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

	// currentReferences is the range of each LSP documentHighlights under the cursor
	// It is used to avoid updating the text property when the cursor is moved within the
	// existing highlights.
	currentReferences []*types.Range

	// cancelDocHighlight is the function to cancel the ongoing LSP documentHighlight call. It must
	// be called before assigning a new value (or nil) to it. It is nil when there is no ongoing
	// call.
	cancelDocHighlight     context.CancelFunc
	cancelDocHighlightLock sync.Mutex

	bufferUpdates chan *bufferUpdate

	// inShutdown is closed when govim is told to Shutdown
	inShutdown chan struct{}
}

func newplugin(goplspath string, goplsEnv []string, defaults, user *config.Config) *govimplugin {
	if goplsEnv == nil {
		goplsEnv = os.Environ()
	}
	tmpDir := getEnvVal(goplsEnv, "TMPDIR")
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	if defaults == nil {
		defaults = &config.Config{
			FormatOnSave:            vimconfig.FormatOnSaveVal(config.FormatOnSaveGoImportsGoFmt),
			QuickfixAutoDiagnostics: vimconfig.BoolVal(true),
			QuickfixSigns:           vimconfig.BoolVal(true),
			Staticcheck:             vimconfig.BoolVal(false),
			HighlightDiagnostics:    vimconfig.BoolVal(true),
			HighlightReferences:     vimconfig.BoolVal(true),
			HoverDiagnostics:        vimconfig.BoolVal(true),
			TempModfile:             vimconfig.BoolVal(false),
		}
	}
	// Overlay the initial user values on the defaults
	if user != nil {
		defaults.Apply(user)
	}
	d := plugin.NewDriver(PluginPrefix)
	var emptyDiags []types.Diagnostic
	res := &govimplugin{
		tmpDir:           tmpDir,
		rawDiagnostics:   make(map[span.URI]*protocol.PublishDiagnosticsParams),
		goplsEnv:         goplsEnv,
		goplspath:        goplspath,
		Driver:           d,
		inShutdown:       make(chan struct{}),
		diagnosticsCache: &emptyDiags,
		vimstate: &vimstate{
			Driver:                d,
			buffers:               make(map[int]*types.Buffer),
			defaultConfig:         *defaults,
			config:                *defaults,
			quickfixIsDiagnostics: true,
			suggestedFixesPopups:  make(map[int][]protocol.WorkspaceEdit),
		},
	}
	res.vimstate.govimplugin = res
	return res
}

func (g *govimplugin) Init(gg govim.Govim, errCh chan error) error {
	g.Driver.Govim = gg
	g.vimstate.Driver.Govim = gg.Scheduled()
	g.vimstate.workingDirectory = g.ParseString(g.ChannelCall("getcwd", -1))
	g.DefineFunction(string(config.FunctionBalloonExpr), []string{}, g.vimstate.balloonExpr)
	g.DefineAutoCommand("", govim.Events{govim.EventBufNew}, govim.Patterns{"*"}, false, g.vimstate.bufNew, exprBufNew)
	g.DefineAutoCommand("", govim.Events{govim.EventBufWinEnter}, govim.Patterns{"*"}, false, g.vimstate.bufWinEnter, exprBufWinEnter)
	g.DefineAutoCommand("", govim.Events{govim.EventBufWritePre}, govim.Patterns{"*"}, false, g.vimstate.bufWritePre, "eval(expand('<abuf>'))")
	g.DefineAutoCommand("", govim.Events{govim.EventBufWritePost}, govim.Patterns{"*"}, false, g.vimstate.bufWritePost, "eval(expand('<abuf>'))")
	g.DefineAutoCommand("", govim.Events{govim.EventBufUnload}, govim.Patterns{"*"}, false, g.vimstate.bufUnload, "eval(expand('<abuf>'))")
	g.DefineAutoCommand("", govim.Events{govim.EventBufDelete}, govim.Patterns{"*"}, false, g.vimstate.bufDelete, "eval(expand('<abuf>'))")
	g.DefineAutoCommand("", govim.Events{govim.EventBufWipeout}, govim.Patterns{"*"}, false, g.vimstate.bufWipeout, "eval(expand('<abuf>'))")
	g.DefineFunction(string(config.FunctionComplete), []string{"findarg", "base"}, g.vimstate.complete)
	g.DefineCommand(string(config.CommandGoToDef), g.vimstate.gotoDef, govim.NArgsZeroOrOne)
	g.DefineCommand(string(config.CommandSuggestedFixes), g.vimstate.suggestFixes, govim.NArgsZeroOrOne)
	g.DefineCommand(string(config.CommandGoToPrevDef), g.vimstate.gotoPrevDef, govim.NArgsZeroOrOne, govim.CountN(1))
	g.DefineFunction(string(config.FunctionHover), []string{}, g.vimstate.hover)
	g.DefineCommand(string(config.CommandGoFmt), g.vimstate.gofmtCurrentBufferRange)
	g.DefineCommand(string(config.CommandGoImports), g.vimstate.goimportsCurrentBufferRange)
	g.DefineCommand(string(config.CommandQuickfixDiagnostics), g.vimstate.quickfixDiagnostics)
	g.DefineFunction(string(config.FunctionBufChanged), []string{"bufnr", "start", "end", "added", "changes"}, g.vimstate.bufChanged)
	g.DefineFunction(string(config.FunctionSetConfig), []string{"config"}, g.vimstate.setConfig)
	g.ChannelExf(`call govim#config#Set("%vFunc", function("%v%v"))`, config.InternalFunctionPrefix, PluginPrefix, config.FunctionSetConfig)
	g.DefineFunction(string(config.FunctionSetUserBusy), []string{"isBusy"}, g.vimstate.setUserBusy)
	g.DefineFunction(string(config.FunctionPopupSelection), []string{"id", "selected"}, g.vimstate.popupSelection)
	g.DefineCommand(string(config.CommandReferences), g.vimstate.references)
	g.DefineCommand(string(config.CommandRename), g.vimstate.rename, govim.NArgsZeroOrOne)
	g.DefineCommand(string(config.CommandStringFn), g.vimstate.stringfns, govim.RangeLine, govim.CompleteCustomList(PluginPrefix+config.FunctionStringFnComplete), govim.NArgsOneOrMore)
	g.DefineFunction(string(config.FunctionStringFnComplete), []string{"ArgLead", "CmdLine", "CursorPos"}, g.vimstate.stringfncomplete)
	g.DefineCommand(string(config.CommandHighlightReferences), g.vimstate.referenceHighlight)
	g.DefineAutoCommand("", govim.Events{govim.EventCompleteDone}, govim.Patterns{"*"}, false, g.vimstate.completeDone, "eval(expand('<abuf>'))", "v:completed_item")
	g.DefineAutoCommand("", govim.Events{govim.EventUser}, govim.Patterns{"PostInitComplete"}, true, g.vimstate.postInitComplete, postInitCompleteExpr)
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

	logfile, err := g.createLogFile("gopls")
	if err != nil {
		return err
	}
	logfile.Close()
	g.Logf("gopls log file: %v", logfile.Name())

	g.ChannelExf("let s:gopls_logfile=%q", logfile.Name())

	goplsArgs := []string{"-rpc.trace", "-logfile", logfile.Name()}
	if flags, err := util.Split(os.Getenv(string(config.EnvVarGoplsFlags))); err != nil {
		g.Logf("invalid env var %s: %v", config.EnvVarGoplsFlags, err)
	} else {
		goplsArgs = append(goplsArgs, flags...)
	}

	gopls := exec.Command(g.goplspath, goplsArgs...)
	gopls.Env = g.goplsEnv
	g.Logf("Running gopls: %v", strings.Join(gopls.Args, " "))
	stderr, err := gopls.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe for gopls: %v", err)
	}
	g.tomb.Go(func() error {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			g.Logf("gopls stderr: %v", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading standard input: %v", err)
		}
		return nil
	})
	stdout, err := gopls.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe for gopls: %v", err)
	}
	stdin, err := gopls.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe for gopls: %v", err)
	}
	g.goplsStdin = stdin
	if err := gopls.Start(); err != nil {
		return fmt.Errorf("failed to start gopls: %v", err)
	}
	g.tomb.Go(func() (err error) {
		if err = gopls.Wait(); err != nil {
			err = fmt.Errorf("got error running gopls: %v", err)
		}
		select {
		case <-g.inShutdown:
			return nil
		default:
			if err != nil {
				errCh <- err
			}
			return
		}
	})

	stream := jsonrpc2.NewHeaderStream(stdout, stdin)
	ctxt, cancel := context.WithCancel(context.Background())
	conn := jsonrpc2.NewConn(stream)
	server := protocol.ServerDispatcher(conn)
	conn.AddHandler(protocol.ClientHandler(g))
	conn.AddHandler(protocol.Canceller{})
	ctxt = protocol.WithClient(ctxt, g)

	g.tomb.Go(func() error {
		return conn.Run(ctxt)
	})

	g.gopls = gopls.Process
	g.goplsConn = conn
	g.goplsCancel = cancel
	g.server = loggingGoplsServer{
		u: server,
		g: g,
	}

	initParams := &protocol.ParamInitialize{}
	initParams.RootURI = protocol.DocumentURI(span.URIFromPath(g.vimstate.workingDirectory))
	initParams.Capabilities.TextDocument.Hover = protocol.HoverClientCapabilities{
		ContentFormat: []protocol.MarkupKind{protocol.PlainText},
	}
	initParams.Capabilities.Workspace.Configuration = true
	// TODO: actually handle these registrations dynamically, if we ever want to
	// target language servers other than gopls.
	initParams.Capabilities.Workspace.DidChangeConfiguration.DynamicRegistration = true
	initParams.Capabilities.Workspace.DidChangeWatchedFiles.DynamicRegistration = true
	initOpts := make(map[string]interface{})
	initOpts[goplsInitOptIncrementalSync] = true
	initOpts["noDocsOnHover"] = true
	initParams.InitializationOptions = initOpts

	if _, err := g.server.Initialize(context.Background(), initParams); err != nil {
		return fmt.Errorf("failed to initialise gopls: %v", err)
	}

	if err := g.server.Initialized(context.Background(), &protocol.InitializedParams{}); err != nil {
		return fmt.Errorf("failed to call gopls.Initialized: %v", err)
	}

	// Temporary fix for the fact that gopls does not yet support watching (via
	// the client) changed files: https://github.com/golang/go/issues/31553
	gomodpath, err := goModPath(g.vimstate.workingDirectory)
	if err != nil {
		return fmt.Errorf("failed to derive go.mod path: %v", err)
	}

	if gomodpath != "" {
		// i.e. we are in a module
		mw, err := newModWatcher(g, gomodpath)
		if err != nil {
			return fmt.Errorf("failed to create modWatcher for %v: %v", gomodpath, err)
		}
		g.modWatcher = mw
	}

	return nil
}

const postInitCompleteExpr = `map(getbufinfo(), {_, v -> {'Num': v.bufnr, 'Name': v.name != "" ? fnamemodify(v.name, ':p') : "", 'Contents': join(getbufline(v.bufnr, 0, "$"), "\n")."\n", 'Loaded': bufloaded(v.bufnr) }})`

// postInitComplete is a manually triggered User autocommand that signals
// Vim's acknowledgement that govim has finished initialising.
func (v *vimstate) postInitComplete(args ...json.RawMessage) error {
	var bufInfos []bufWinEnterDetails
	v.Parse(args[0], &bufInfos)
	for _, b := range bufInfos {
		v.bufNewImpl(bufNewDetails{
			Num:  b.Num,
			Name: b.Name,
		})
	}
	for _, b := range bufInfos {
		if b.Loaded == 1 {
			v.bufWinEnterImpl(b)
		}
	}
	return nil
}

func (v *vimstate) strictVimBufferLifecycle() bool {
	return os.Getenv(testsetup.EnvStrictVimBufferLifecycle) == "true"
}

func goModPath(wd string) (string, error) {
	cmd := exec.Command("go", "env", "GOMOD")
	cmd.Dir = wd

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute [%v] in %v: %v\n%s", strings.Join(cmd.Args, " "), wd, err, out)
	}

	return strings.TrimSpace(string(out)), nil
}

func (g *govimplugin) Shutdown() error {
	close(g.inShutdown)
	close(g.bufferUpdates)
	if err := g.server.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("failed to call gopls Shutdown: %v", err)
	}
	// We "kill" gopls by closing its stdin. Standard practice for processes
	// that communicate over stdin/stdout is to exit cleanly when stdin is
	// closed.
	if err := g.goplsStdin.Close(); err != nil {
		return fmt.Errorf("failed to close gopls stdin: %v", err)
	}
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
	} {
		g.vimstate.BatchChannelCall("execute", hi)
	}
	g.vimstate.MustBatchEnd()
}

func getEnvVal(env []string, varname string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], varname+"=") {
			return strings.TrimPrefix(env[i], varname+"=")
		}
	}
	return ""
}
