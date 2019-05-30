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
	"time"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/config"
	"github.com/myitcv/govim/cmd/govim/internal/jsonrpc2"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/span"
	"github.com/myitcv/govim/cmd/govim/types"
	"github.com/myitcv/govim/internal/plugin"
	"github.com/myitcv/govim/testsetup"
	"gopkg.in/tomb.v2"

	"github.com/rogpeppe/go-internal/semver"
)

var (
	fTail = flag.Bool("tail", false, "whether to also log output to stdout")

	// gopls define InitializationOptions; they will make these well defined
	// constants at some point
	goplsInitOptIncrementalSync = "incrementalSync"
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

	var tf *os.File
	var err error
	logfiletmpl := os.Getenv(testsetup.EnvLogfileTmpl)
	if logfiletmpl == "" {
		logfiletmpl = "%v_%v_%v"
	}
	logfiletmpl = strings.Replace(logfiletmpl, "%v", "govim_log", 1)
	logfiletmpl = strings.Replace(logfiletmpl, "%v", time.Now().Format("20060102_1504_05.000000000"), 1)
	if strings.Contains(logfiletmpl, "%v") {
		logfiletmpl = strings.Replace(logfiletmpl, "%v", "*", 1)
		tf, err = ioutil.TempFile("", logfiletmpl)

	} else {
		// append to existing file
		tf, err = os.OpenFile(filepath.Join(os.TempDir(), logfiletmpl), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	}
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
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

type govimplugin struct {
	plugin.Driver
	*vimstate

	goplspath   string
	gopls       *os.Process
	goplsConn   *jsonrpc2.Conn
	goplsCancel context.CancelFunc
	server      protocol.Server

	isGui bool

	tomb tomb.Tomb

	modWatcher *modWatcher
}

func newplugin(goplspath string) *govimplugin {
	d := plugin.NewDriver("GOVIM")
	res := &govimplugin{
		goplspath: goplspath,
		Driver:    d,
		vimstate: &vimstate{
			Driver:       d,
			buffers:      make(map[int]*types.Buffer),
			watchedFiles: make(map[string]*types.WatchedFile),
			diagnostics:  make(map[span.URI][]protocol.Diagnostic),
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
	g.DefineFunction(string(config.FunctionHello), []string{}, g.hello)
	g.DefineCommand(string(config.CommandHello), g.helloComm)
	g.DefineFunction(string(config.FunctionBalloonExpr), []string{}, g.balloonExpr)
	g.DefineAutoCommand("", govim.Events{govim.EventBufRead, govim.EventBufNewFile}, govim.Patterns{"*.go"}, false, g.bufReadPost, exprAutocmdCurrBufInfo)
	if !g.doIncrementalSync() {
		g.DefineAutoCommand("", govim.Events{govim.EventTextChanged, govim.EventTextChangedI}, govim.Patterns{"*.go"}, false, g.bufTextChanged, exprAutocmdCurrBufInfo)
	}
	g.DefineAutoCommand("", govim.Events{govim.EventBufWritePre}, govim.Patterns{"*.go"}, false, g.formatCurrentBuffer, "eval(expand('<abuf>'))")
	g.DefineFunction(string(config.FunctionComplete), []string{"findarg", "base"}, g.complete)
	g.DefineCommand(string(config.CommandGoToDef), g.gotoDef, govim.NArgsZeroOrOne)
	g.DefineCommand(string(config.CommandGoToPrevDef), g.gotoPrevDef, govim.NArgsZeroOrOne, govim.CountN(1))
	g.DefineFunction(string(config.FunctionHover), []string{}, g.hover)
	g.DefineAutoCommand("", govim.Events{govim.EventCursorHold, govim.EventCursorHoldI}, govim.Patterns{"*.go"}, false, g.autoUpdateQuickfix)
	g.DefineAutoCommand("", govim.Events{govim.EventBufDelete}, govim.Patterns{"*.go"}, false, g.deleteCurrentBuffer, "eval(expand('<abuf>'))")
	g.DefineCommand(string(config.CommandGoFmt), g.gofmtCurrentBufferRange, govim.RangeFile)
	g.DefineCommand(string(config.CommandGoImports), g.goimportsCurrentBufferRange, govim.RangeFile)
	g.DefineCommand(string(config.CommandQuickfixDiagnostics), g.quickfixDiagnostics)
	g.DefineFunction(string(config.FunctionBufChanged), []string{"bufnr", "start", "end", "added", "changes"}, g.bufChanged)

	g.isGui = g.ParseInt(g.ChannelExpr(`has("gui_running")`)) == 1

	gopls := exec.Command(g.goplspath)
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
	conn, server, _ := protocol.NewClient(stream, g)
	// override the handler with something that can handle the fact
	// that we might get a govim.ErrShuttingDown
	currHandler := conn.Handler
	conn.Handler = func(ctxt context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
		defer func() {
			if r := recover(); r != nil && r != govim.ErrShuttingDown {
				panic(r)
			}
		}()
		currHandler(ctxt, conn, req)
	}
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
	if os.Getenv(testsetup.EnvDisableIncrementalSync) == "false" {
		return true
	}
	return false
}
