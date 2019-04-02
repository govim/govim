// Command govim is a Vim8 channel-based plugin, written in Go, to support the writing of Go code in Vim8
package main

import (
	"context"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kr/pretty"
	"github.com/myitcv/govim"
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
				if err := launch(conn, conn); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}()
		}
	} else {
		return launch(os.Stdin, os.Stdout)
	}
}

func launch(in io.ReadCloser, out io.WriteCloser) error {
	defer out.Close()

	d := newplugin()

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

	gopls       *os.Process
	goplsConn   *jsonrpc2.Conn
	goplsCancel context.CancelFunc
	server      protocol.Server

	tomb tomb.Tomb
}

type jumpPos struct {
	WinID int
	BufNr int
	Line  int
	Col   int
}

type parseData struct {
	fset *token.FileSet
	file *ast.File
}

func newplugin() *govimplugin {
	d := plugin.NewDriver("GOVIM")
	res := &govimplugin{
		Driver: d,
		vimstate: &vimstate{
			Driver:    d,
			buffers:   make(map[int]*types.Buffer),
			jumpStack: make(map[int][]jumpPos),
		},
	}
	res.vimstate.govimplugin = res
	return res
}

func (g *govimplugin) Init(gg govim.Govim) error {
	g.Driver.Govim = gg
	g.vimstate.Driver.Govim = gg.Sync()
	g.ChannelEx(`augroup govim`)
	g.ChannelEx(`augroup END`)
	g.DefineFunction("Hello", []string{}, g.hello)
	g.DefineCommand("Hello", g.helloComm)
	g.DefineFunction("BalloonExpr", []string{}, g.balloonExpr)
	g.ChannelEx("set balloonexpr=GOVIMBalloonExpr()")
	g.DefineAutoCommand("", govim.Events{govim.EventBufReadPost, govim.EventBufNewFile}, govim.Patterns{"*.go"}, false, g.bufReadPost)
	g.DefineAutoCommand("", govim.Events{govim.EventTextChanged, govim.EventTextChangedI}, govim.Patterns{"*.go"}, false, g.bufTextChanged)
	g.DefineAutoCommand("", govim.Events{govim.EventBufWritePre}, govim.Patterns{"*.go"}, false, g.formatCurrentBuffer)
	g.DefineFunction("Complete", []string{"findarg", "base"}, g.complete)
	g.ChannelEx("set omnifunc=GOVIMComplete")

	goplsPath, err := installGoPls()
	if err != nil {
		return fmt.Errorf("failed to install gopls: %v", err)
	}

	gopls := exec.Command(goplsPath)
	out, err := gopls.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe for gopls: %v", err)
	}
	in, err := gopls.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe for gopls: %v", err)
	}
	if err := gopls.Start(); err != nil {
		return fmt.Errorf("failed to start gopls: %v", err)
	}
	g.tomb.Go(func() (err error) {
		if err = gopls.Wait(); err != nil {
			err = fmt.Errorf("got error running gopls: %v", err)
		}
		return
	})

	stream := jsonrpc2.NewHeaderStream(out, in)
	ctxt, cancel := context.WithCancel(context.Background())
	conn, server := protocol.NewClient(stream, g)
	go conn.Run(ctxt)

	g.gopls = gopls.Process
	g.goplsConn = conn
	g.goplsCancel = cancel
	g.server = server

	wd := g.ParseString(g.ChannelCall("getcwd", -1))
	initParams := &protocol.InitializeParams{
		InnerInitializeParams: protocol.InnerInitializeParams{
			RootURI: string(span.FileURI(wd)),
		},
	}
	g.Logf("calling protocol.Initialize(%v)", pretty.Sprint(initParams))
	initRes, err := server.Initialize(context.Background(), initParams)
	if err != nil {
		return fmt.Errorf("failed to initialise gopls: %v", err)
	}
	g.Logf("gopls init complete: %v", pretty.Sprint(initRes.Capabilities))

	return nil
}

func (s *govimplugin) Shutdown() error {
	return nil
}

func installGoPls() (string, error) {
	// If we are being run as a plugin we require that it is somewhere within
	// the github.com/myitcv/govim module. That allows tests to work but also
	// the plugin itself when run from within plugin/govim.vim
	modlist := exec.Command("go", "list", "-m", "-f={{.Dir}}", "github.com/myitcv/govim")
	out, err := modlist.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to determine directory of github.com/myitcv/govim: %v", err)
	}

	gobin := filepath.Join(string(out), "cmd", "govim", ".bin")

	cmd := exec.Command("go", "install", "golang.org/x/tools/cmd/gopls")
	cmd.Env = append(os.Environ(), "GOBIN="+gobin)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run [%v] in %v: %v\n%s", strings.Join(cmd.Args, " "), gobin, err, out)
	}

	return filepath.Join(gobin, "gopls"), nil
}
