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

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/config"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/jsonrpc2"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/myitcv/govim/cmd/govim/internal/types"
	"github.com/myitcv/govim/internal/plugin"
	"github.com/myitcv/govim/testsetup"
	"gopkg.in/tomb.v2"

	"github.com/rogpeppe/go-internal/semver"
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

	args := flag.Args()
	if len(flag.Args()) == 0 {
		return fmt.Errorf("missing single argument path to gopls")
	}
	goplspath := args[0]

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
				if err := launch(goplspath, conn, conn); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}()
		}
	} else {
		return launch(goplspath, os.Stdin, os.Stdout)
	}
}

func launch(goplspath string, in io.ReadCloser, out io.WriteCloser) error {
	defer out.Close()

	d := newplugin(goplspath)

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

	// diagnostics gives us the current diagnostics by URI
	diagnostics     map[span.URI][]protocol.Diagnostic
	diagnosticsLock sync.Mutex

	bufferUpdates chan *bufferUpdate
}

func newplugin(goplspath string) *govimplugin {
	d := plugin.NewDriver(PluginPrefix)
	res := &govimplugin{
		diagnostics: make(map[span.URI][]protocol.Diagnostic),
		goplspath:   goplspath,
		Driver:      d,
		vimstate: &vimstate{
			Driver:                d,
			buffers:               make(map[int]*types.Buffer),
			watchedFiles:          make(map[string]*types.WatchedFile),
			quickfixIsDiagnostics: true,
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
	g.DefineAutoCommand("", govim.Events{govim.EventBufRead, govim.EventBufNewFile}, govim.Patterns{"*.go"}, false, g.vimstate.bufReadPost, exprAutocmdCurrBufInfo)
	if !g.doIncrementalSync() {
		g.DefineAutoCommand("", govim.Events{govim.EventTextChanged, govim.EventTextChangedI}, govim.Patterns{"*.go"}, false, g.vimstate.bufTextChanged, exprAutocmdCurrBufInfo)
	}
	g.DefineAutoCommand("", govim.Events{govim.EventBufWritePre}, govim.Patterns{"*.go"}, false, g.vimstate.formatCurrentBuffer, "eval(expand('<abuf>'))")
	g.DefineFunction(string(config.FunctionComplete), []string{"findarg", "base"}, g.vimstate.complete)
	g.DefineCommand(string(config.CommandGoToDef), g.vimstate.gotoDef, govim.NArgsZeroOrOne)
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

	gopls := exec.Command(g.goplspath, "-rpc.trace", "-logfile", logfile.Name())
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
	initParams := &protocol.InitializeParams{}
	initParams.RootURI = string(span.FileURI(wd))
	initParams.Capabilities.TextDocument.Hover.ContentFormat = []protocol.MarkupKind{protocol.PlainText}
	initParams.Capabilities.Workspace.Configuration = true
	initParams.Capabilities.Workspace.DidChangeConfiguration.DynamicRegistration = true
	initOpts := make(map[string]interface{})
	if g.doIncrementalSync() {
		initOpts[goplsInitOptIncrementalSync] = true
	}
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

func (g *govimplugin) doIncrementalSync() bool {
	if g.Flavor() != govim.FlavorVim && g.Flavor() != govim.FlavorGvim {
		return false
	}
	if semver.Compare(g.Version(), testsetup.MinVimIncrementalSync) < 0 {
		return false
	}
	if os.Getenv(testsetup.EnvDisableIncrementalSync) == "true" {
		return false
	}
	return true
}

func (g *govimplugin) usePopupWindows() bool {
	if g.Flavor() != govim.FlavorVim && g.Flavor() != govim.FlavorGvim {
		return false
	}
	if semver.Compare(g.Version(), testsetup.MinPopupWindowBalloon) < 0 {
		return false
	}
	if os.Getenv(testsetup.EnvDisablePopupWindowBalloon) == "true" {
		return false
	}
	return true
}

func (g *govimplugin) placeSigns() bool {
	if g.Flavor() != govim.FlavorVim && g.Flavor() != govim.FlavorGvim {
		return false
	}
	if semver.Compare(g.Version(), testsetup.MinSignPlace) < 0 {
		return false
	}
	if os.Getenv(testsetup.EnvDisableSignPlace) == "true" {
		return false
	}
	return true
}
