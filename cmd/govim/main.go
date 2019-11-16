// Command govim is a Vim8 channel-based plugin, written in Go, to support the writing of Go code in Vim8
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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

	d := newplugin(goplspath, nil)

	tf, err := createLogFile("govim_log")
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

	g, err := govim.NewGovim(d, in, out, log)
	if err != nil {
		return fmt.Errorf("failed to create govim instance: %v", err)
	}

	d.tomb.Kill(g.Run())
	return d.tomb.Wait()
}

func createLogFile(prefix string) (*os.File, error) {
	var tf *os.File
	var err error
	logfiletmpl := os.Getenv(testsetup.EnvLogfileTmpl)
	if logfiletmpl == "" {
		logfiletmpl = "%v_%v_%v"
	}
	logfiletmpl = strings.Replace(logfiletmpl, "%v", prefix, 1)
	logfiletmpl = strings.Replace(logfiletmpl, "%v", time.Now().Format("20060102_1504_05"), 1)
	if strings.Contains(logfiletmpl, "%v") {
		logfiletmpl = strings.Replace(logfiletmpl, "%v", "*", 1)
		tf, err = ioutil.TempFile("", logfiletmpl)
	} else {
		// append to existing file
		tf, err = os.OpenFile(filepath.Join(os.TempDir(), logfiletmpl), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	}
	if err != nil {
		err = fmt.Errorf("failed to create log file: %v", err)
	}
	return tf, err
}

type govimplugin struct {
	plugin.Driver
	vimstate *vimstate

	goplspath   string
	gopls       *os.Process
	goplsConn   *jsonrpc2.Conn
	goplsCancel context.CancelFunc
	server      protocol.Server

	isGui bool

	tomb tomb.Tomb

	modWatcher *modWatcher

	// rawDiagnostics holds the current raw (LSP) diagnostics by URI
	rawDiagnostics     map[span.URI]*protocol.PublishDiagnosticsParams
	rawDiagnosticsLock sync.Mutex

	// diagnosticsCache isn't inteded to be used directly since it might
	// contain old data. Call diagnostics() to get the latest instead.
	diagnosticsCache []types.Diagnostic

	bufferUpdates chan *bufferUpdate
}

func newplugin(goplspath string, defaults *config.Config) *govimplugin {
	if defaults == nil {
		defaults = &config.Config{
			FormatOnSave:            vimconfig.FormatOnSaveVal(config.FormatOnSaveGoImports),
			QuickfixAutoDiagnostics: vimconfig.BoolVal(true),
			QuickfixSigns:           vimconfig.BoolVal(true),
			Staticcheck:             vimconfig.BoolVal(false),
		}
	}
	d := plugin.NewDriver(PluginPrefix)
	res := &govimplugin{
		rawDiagnostics: make(map[span.URI]*protocol.PublishDiagnosticsParams),
		goplspath:      goplspath,
		Driver:         d,
		vimstate: &vimstate{
			Driver:                d,
			buffers:               make(map[int]*types.Buffer),
			defaultConfig:         *defaults,
			config:                *defaults,
			watchedFiles:          make(map[string]*types.WatchedFile),
			quickfixIsDiagnostics: true,
			suggestedFixesPopups:  make(map[int][]*protocol.WorkspaceEdit),
		},
	}
	res.vimstate.govimplugin = res
	return res
}

func (g *govimplugin) Init(gg govim.Govim, errCh chan error) error {
	g.Driver.Govim = gg
	g.vimstate.Driver.Govim = gg.Scheduled()
	g.ChannelEx(`augroup govim`)
	g.ChannelEx(`augroup END`)
	g.DefineFunction(string(config.FunctionBalloonExpr), []string{}, g.vimstate.balloonExpr)
	g.DefineAutoCommand("", govim.Events{govim.EventBufUnload}, govim.Patterns{"*.go"}, false, g.vimstate.bufUnload, "eval(expand('<abuf>'))")
	g.DefineAutoCommand("", govim.Events{govim.EventBufRead, govim.EventBufNewFile}, govim.Patterns{"*.go"}, false, g.vimstate.bufReadPost, exprAutocmdCurrBufInfo)
	g.DefineAutoCommand("", govim.Events{govim.EventBufWritePre}, govim.Patterns{"*.go"}, false, g.vimstate.formatCurrentBuffer, "eval(expand('<abuf>'))")
	g.DefineFunction(string(config.FunctionComplete), []string{"findarg", "base"}, g.vimstate.complete)
	g.DefineCommand(string(config.CommandGoToDef), g.vimstate.gotoDef, govim.NArgsZeroOrOne)
	g.DefineCommand(string(config.CommandSuggestedFixes), g.vimstate.suggestFixes, govim.NArgsZeroOrOne)
	g.DefineCommand(string(config.CommandGoToPrevDef), g.vimstate.gotoPrevDef, govim.NArgsZeroOrOne, govim.CountN(1))
	g.DefineFunction(string(config.FunctionHover), []string{}, g.vimstate.hover)
	g.DefineAutoCommand("", govim.Events{govim.EventBufDelete}, govim.Patterns{"*.go"}, false, g.vimstate.deleteCurrentBuffer, "eval(expand('<abuf>'))")
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
	if g.placeSigns() {
		if err := g.vimstate.signDefine(); err != nil {
			return fmt.Errorf("failed to define signs: %v", err)
		}
	}
	g.DefineFunction(string(config.FunctionMotion), []string{"direction", "target"}, g.vimstate.motion)

	g.startProcessBufferUpdates()

	g.InitTestAPI()

	g.isGui = g.ParseInt(g.ChannelExpr(`has("gui_running")`)) == 1

	logfile, err := createLogFile("gopls_log")
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
	if err := gopls.Start(); err != nil {
		return fmt.Errorf("failed to start gopls: %v", err)
	}
	g.tomb.Go(func() (err error) {
		if err = gopls.Wait(); err != nil {
			err = fmt.Errorf("got error running gopls: %v", err)
			errCh <- err
		}
		return
	})

	stream := jsonrpc2.NewHeaderStream(stdout, stdin)
	ctxt, cancel := context.WithCancel(context.Background())
	ctxt, conn, server := protocol.NewClient(ctxt, stream, g)
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

	wd := g.ParseString(g.ChannelCall("getcwd", -1))
	initParams := &protocol.ParamInitia{}
	initParams.RootURI = string(span.FileURI(wd))
	initParams.Capabilities.TextDocument.Hover = &protocol.HoverClientCapabilities{
		ContentFormat: []protocol.MarkupKind{protocol.PlainText},
	}
	initParams.Capabilities.Workspace.Configuration = true
	initParams.Capabilities.Workspace.DidChangeConfiguration.DynamicRegistration = true
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
	gomodpath, err := goModPath(wd)
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
	close(g.bufferUpdates)
	if err := g.server.Shutdown(context.Background()); err != nil {
		return err
	}
	if g.modWatcher != nil {
		if err := g.modWatcher.close(); err != nil {
			return err
		}
	}
	return nil
}

func (g *govimplugin) placeSigns() bool {
	if g.Flavor() != govim.FlavorVim && g.Flavor() != govim.FlavorGvim {
		return false
	}
	if os.Getenv(testsetup.EnvDisableSignPlace) == "true" {
		return false
	}
	return true
}
