package main

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/jsonrpc2"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/util"
)

func (g *govimplugin) startGopls() error {
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
	if ev, ok := os.LookupEnv(string(config.EnvVarGoplsGOMAXPROCSMinusN)); ok {
		v := strings.TrimSpace(ev)
		var gmp int
		if strings.HasSuffix(v, "%") {
			v = strings.TrimSuffix(v, "%")
			p, err := strconv.ParseFloat(v, 10)
			if err != nil {
				return fmt.Errorf("failed to parse percentage from %v value %q: %v", config.EnvVarGoplsGOMAXPROCSMinusN, ev, err)
			}
			gmp = int(math.Floor(float64(runtime.NumCPU()) * (1 - p/100)))
		} else {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("failed to parse integer from %v value %q: %v", config.EnvVarGoplsGOMAXPROCSMinusN, ev, err)
			}
			gmp = runtime.NumCPU() - n
		}
		if gmp < 0 || gmp > runtime.NumCPU() {
			return fmt.Errorf("%v value %q results in GOMAXPROCS value %v which is invalid", config.EnvVarGoplsGOMAXPROCSMinusN, ev, gmp)
		}
		g.Logf("Starting gopls with GOMAXPROCS=%v", gmp)
		gopls.Env = append(gopls.Env, "GOMAXPROCS="+strconv.Itoa(gmp))
	}
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
				g.errCh <- err
			}
			return
		}
	})

	stream := jsonrpc2.NewHeaderStream(stdout, stdin)
	ctxt, cancel := context.WithCancel(context.Background())
	conn := jsonrpc2.NewConn(stream)
	server := protocol.ServerDispatcher(conn)
	handler := protocol.ClientHandler(g, jsonrpc2.MethodNotFound)
	handler = protocol.Handlers(handler)
	ctxt = protocol.WithClient(ctxt, g)

	g.tomb.Go(func() error {
		return conn.Run(ctxt, handler)
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

	if _, err := g.server.Initialize(context.Background(), initParams); err != nil {
		return fmt.Errorf("failed to initialise gopls: %v", err)
	}

	if err := g.server.Initialized(context.Background(), &protocol.InitializedParams{}); err != nil {
		return fmt.Errorf("failed to call gopls.Initialized: %v", err)
	}

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
