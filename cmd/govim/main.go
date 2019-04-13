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
	"time"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/config"
	"github.com/myitcv/govim/cmd/govim/internal/jsonrpc2"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/span"
	"github.com/myitcv/govim/cmd/govim/types"
	"github.com/myitcv/govim/internal/plugin"
	"gopkg.in/tomb.v2"
)

var (
	fTail = flag.Bool("tail", false, "whether to also log output to stdout")
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

	if sock := os.Getenv("GOVIMTEST_SOCKET"); sock != "" {
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

	nowStr := time.Now().Format("20060102_1504_05.000000000")
	tf, err := ioutil.TempFile("", "govim_"+nowStr+"_*")
	if err != nil {
		return fmt.Errorf("failed to create log file")
	}
	defer tf.Close()

	var log io.Writer = tf
	if *fTail {
		log = io.MultiWriter(tf, os.Stdout)
	}

	if os.Getenv("GOVIMTEST_SOCKET") != "" {
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
}

func newplugin(goplspath string) *govimplugin {
	d := plugin.NewDriver("GOVIM")
	res := &govimplugin{
		goplspath: goplspath,
		Driver:    d,
		vimstate: &vimstate{
			Driver:  d,
			buffers: make(map[int]*types.Buffer),
		},
	}
	res.vimstate.govimplugin = res
	return res
}

func (g *govimplugin) Init(gg govim.Govim, errCh chan error) error {
	g.Driver.Govim = gg
	g.vimstate.Driver.Govim = gg.Sync()
	g.ChannelEx(`augroup govim`)
	g.ChannelEx(`augroup END`)
	g.DefineFunction(string(config.FunctionHello), []string{}, g.hello)
	g.DefineCommand(string(config.CommandHello), g.helloComm)
	g.DefineFunction(string(config.FunctionBalloonExpr), []string{}, g.balloonExpr)
	g.DefineAutoCommand("", govim.Events{govim.EventBufRead, govim.EventBufNewFile}, govim.Patterns{"*.go"}, false, g.bufReadPost)
	g.DefineAutoCommand("", govim.Events{govim.EventTextChanged, govim.EventTextChangedI}, govim.Patterns{"*.go"}, false, g.bufTextChanged)
	g.DefineAutoCommand("", govim.Events{govim.EventBufWritePre}, govim.Patterns{"*.go"}, false, g.formatCurrentBuffer)
	g.DefineFunction(string(config.FunctionComplete), []string{"findarg", "base"}, g.complete)
	g.DefineCommand(string(config.CommandGoToDef), g.gotoDef, govim.NArgsZeroOrOne)
	g.DefineCommand(string(config.CommandGoToPrevDef), g.gotoPrevDef, govim.NArgsZeroOrOne, govim.CountN(1))

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
	go conn.Run(ctxt)

	g.gopls = gopls.Process
	g.goplsConn = conn
	g.goplsCancel = cancel
	g.server = loggingGoplsServer{
		u: server,
		g: g,
	}

	wd := g.ParseString(g.ChannelCall("getcwd", -1))
	initParams := &protocol.InitializeParams{
		InnerInitializeParams: protocol.InnerInitializeParams{
			RootURI: string(span.FileURI(wd)),
		},
	}
	if _, err := g.server.Initialize(context.Background(), initParams); err != nil {
		return fmt.Errorf("failed to initialise gopls: %v", err)
	}

	return nil
}

func (s *govimplugin) Shutdown() error {
	return nil
}
