package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/kr/pretty"
)

// startParentServer is called during the init phase of cmd/govim. It starts a
// Unix Domain Sockets server to allow for communication from a child instance
func (g *govimplugin) startParentServer() error {
	td, err := ioutil.TempDir("", "govim-parent-child-*")
	if err != nil {
		return fmt.Errorf("failed to create a temp dir for the socket file: %v", err)
	}
	socketFile := filepath.Join(td, "socket")
	g.socketDir = td
	// TODO: support TCP?
	l, err := net.Listen("unix", socketFile)
	if err != nil {
		return fmt.Errorf("failed to listen on %v: %v", socketFile, err)
	}
	g.socketListener = l
	g.parentCallArgs = []string{
		os.Args[0],
		"-parent",
		socketFile,
	}
	g.tomb.Go(g.runParentServer)
	return nil
}

func (g *govimplugin) runParentServer() error {
	for {
		// We assume for now that we will only ever receive sequential calls from
		// a single client, given the main use-case is fzf calling this in
		// response to a user typing input into the Symbol fuzzy finder.
		//
		// If we need to change this assumption we will need to think carefully
		// about sequencing, especially if we start to allow govim commands a la
		// testdriver instead of just pass through calls to gopls endpoints that
		// are known to be side-effect free.
		//
		// Again we keep things simple for now to assume that each accepted
		// connection will handle exactly one request, and therefore send exactly
		// one response.
		conn, err := g.socketListener.Accept()
		if err != nil {
			// TODO: any more definite way of determining that we have safely
			// been shutdown?
			break
		}
		g.Logf("New child-parent request")
		req := &parentReq{
			govimplugin: g,
		}
		if err := req.handle(conn); err != nil {
			g.Logf("child-parent request failed: %v", err)
		}
	}
	g.Logf("Child-parent listener shutdown")
	return nil
}

// parentReq is the root command that is Run in response to a child request.
// It holds the key aspects of handling a query from a child instance. Sub
// commands embed the instance of *parentReq; more generally, sub command
// instances embed parent command instances.
type parentReq struct {
	*govimplugin
	conn    net.Conn
	enc     *json.Encoder
	dec     *json.Decoder
	encLock sync.Mutex
}

// *parentReq implements Command
var _ Command = (*parentReq)(nil)

// knownParentErr is a type used by a "parent" instance of govim to bail out of
// command processing in such a way that the panic-ed error is then returned to
// child as raw output on os.Stderr
type knownParentErr struct {
	err      error
	exitCode int
}

// Errorf implements Command.Errorf
func (p *parentReq) Errorf(format string, args ...interface{}) {
	panic(knownParentErr{err: fmt.Errorf(format, args...)})
}

// Logf is a convenience wrapper around *govimplugin.Logf to make outputting
// of parent-originated log messages easier
func (p *parentReq) Logf(format string, args ...interface{}) {
	p.govimplugin.Logf("parent: "+format, args...)
}

// PrintfStdout encodes the formatted string to be returned for raw
// output to os.Stdout
func (p *parentReq) PrintfStdout(format string, args ...interface{}) {
	p.printf(encodeCodeRawStdout, fmt.Sprintf(format, args...))
}

// PrintfStderr encodes the formatted string to be returned for raw
// output to os.Stderr
func (p *parentReq) PrintfStderr(format string, args ...interface{}) {
	p.printf(encodeCodeRawStderr, fmt.Sprintf(format, args...))
}

func (p *parentReq) printf(dest encodeCode, format string, args ...interface{}) {
	p.encode(dest, fmt.Sprintf(format, args...))
}

// EncodeStdout encodes v to be returned for JSON-encoded output to os.Stdout
func (p *parentReq) EncodeStdout(v interface{}) {
	p.encode(encodeCodeJSONStdout, v)
}

// EncodeStderr encodes v to be returned for JSON-encoded output to os.Stderr
func (p *parentReq) EncodeStderr(v interface{}) {
	p.encode(encodeCodeJSONStderr, v)
}

// encode is the single point of encoding for all responses from parent to
// child. Don't call this directly; call one of the Encode* or Printf*
// methods instead.
func (p *parentReq) encode(dest encodeCode, v interface{}) {
	p.encLock.Lock()
	defer p.encLock.Unlock()
	errorf := func(format string, args ...interface{}) {
		panic(encodeError(fmt.Errorf(format, args...)))
	}
	if err := p.enc.Encode(dest); err != nil {
		errorf("failed to encode response dest: %v", err)
	}
	if err := p.enc.Encode(v); err != nil {
		errorf("failed to encode value: %v", err)
	}
}

// encodeError is used to distinguish encoding errors (that is the parent
// encoding responses to the child) from other errors. If the parent encounters
// an encodeError it is simply ignored because it is assumed the child quit,
// taking the connection with it.
type encodeError error

// handle responds to the client returning a nil-nil error
// only in the case of a fatal communication error with the client
func (p *parentReq) handle(conn net.Conn) (retErr error) {
	p.Logf("handling new connection from child")
	p.conn = conn
	p.enc = json.NewEncoder(conn)
	p.dec = json.NewDecoder(conn)

	// Ignore all encodeError panics
	defer func() {
		switch r := recover().(type) {
		case nil:
		case encodeError:
		default:
			panic(r)
		}
	}()

	// Handle knownParentErr panics by sending the Error() to the child as raw
	// os.Stderr output. Also ensure that in the case of no panics we send the
	// exit code to the child.
	defer func() {
		var err error
		var exitCode int
		switch r := recover().(type) {
		case nil:
		case knownParentErr:
			// A knownError is one that we knowingly threw within
			// a command
			err = r.err
			if r.exitCode != 0 {
				exitCode = r.exitCode
			} else {
				exitCode = -1
			}
		default:
			// This is serious trouble
			panic(r)
		}
		if err != nil {
			p.encode(encodeCodeRawStderr, err.Error())
		}
		p.encode(encodeCodeExitCode, exitCode)
		p.Logf("closing connection")
		p.conn.Close()
	}()

	// We simply run p
	p.Run()
	return
}

// Run implements Command.Run()
func (p *parentReq) Run() {
	var args []string
	if err := p.dec.Decode(&args); err != nil {
		p.Errorf("failed to decode args slice: %v", err)
	}
	p.Logf("parentReq got args: %v", pretty.Sprint(args))

	// At this point args will be something like
	//
	//     gopls Symbol -quickfix govim main
	//
	// where "govim" and "main" are arguments to Symbol and
	// -quickfix is a flag to Symbol.
	if len(args) == 0 {
		p.Errorf("expected command")
	}
	cmdStr := commandName(args[0])

	var cmd Command
	switch cmdStr {
	case cmdNameGopls:
		cmd = newGoplsCmd(p, args[1:])
	default:
		p.Errorf("unknown command: %v", cmdStr)
	}
	cmd.Run()
}

// goplsCmd is a sub Command of parentReq responsible for handling requests
// against the LSP/gopls API
type goplsCmd struct {
	*parentReq
	fs *flag.FlagSet
}

// *goplsCmd implemenets Command
var _ Command = (*goplsCmd)(nil)

func newGoplsCmd(parent *parentReq, args []string) *goplsCmd {
	g := &goplsCmd{
		parentReq: parent,
	}
	g.fs = flag.NewFlagSet("gopls", flag.ContinueOnError)
	if err := g.fs.Parse(args); err != nil {
		g.Errorf("failed to parse args [%v]: %v", strings.Join(args, " "), err)
	}
	return g
}

// Errorf implements Command.Errorf
func (g *goplsCmd) Errorf(format string, args ...interface{}) {
	g.parentReq.Errorf("gopls: "+format, args...)
}

// Run implements Command.Run()
func (g *goplsCmd) Run() {
	args := g.fs.Args()
	g.Logf("goplsCmd got args: %v", pretty.Sprint(args))
	if len(args) == 0 {
		g.Errorf("expected method")
	}
	cmdStr := goplsMethodName(args[0])
	var cmd Command
	switch cmdStr {
	case methodGoplsSymbol:
		cmd = newGoplsSymbolCmd(g, args[1:])
	default:
		g.Errorf("unknown method: %v", cmdStr)
	}
	cmd.Run()
}

// goplsSymbolCmd is a sub Command of goplsCmd responsible for handling a call
// to the gopls Symbol method
type goplsSymbolCmd struct {
	*goplsCmd
	fs        *flag.FlagSet
	fQuickfix *bool
	fRel      *bool
}

func newGoplsSymbolCmd(parent *goplsCmd, args []string) *goplsSymbolCmd {
	g := &goplsSymbolCmd{
		goplsCmd: parent,
	}
	g.fs = flag.NewFlagSet("goplsSymbol", flag.ContinueOnError)
	g.fQuickfix = g.fs.Bool("quickfix", false, "format output in quickfix style")
	g.fRel = g.fs.Bool("rel", false, "output filenames relative to the working directory (only with -quickfix)")
	if err := g.fs.Parse(args); err != nil {
		g.Errorf("failed to parse args [%v]: %v", strings.Join(args, " "), err)
	}
	return g
}

// Errorf implements Command.Errorf
func (g *goplsSymbolCmd) Errorf(format string, args ...interface{}) {
	g.goplsCmd.Errorf("symbol: "+format, args...)
}

// Run implements Command.Run()
func (g *goplsSymbolCmd) Run() {
	g.Logf("goplsSymbolCmd got args: %v", pretty.Sprint(g.fs.Args()))
	if len(g.fs.Args()) == 0 {
		// no error - simply not results
		return
	}
	query := strings.Join(g.fs.Args(), " ")
	symbolReq := &protocol.WorkspaceSymbolParams{
		Query: query,
	}
	symbolResp, err := g.server.Symbol(context.Background(), symbolReq)
	if err != nil {
		g.Errorf("failed to call gopls.Symbol: %v", err)
	}

	if !*g.fQuickfix {
		// we return the raw LSP response. It's highly unlikely this would ever
		// be useful because the locations are in terms of UTF16 code points.
		//
		// See https://github.com/golang/go/issues/38274
		g.EncodeStdout(symbolResp)
		return
	}

	var qfs []quickfixEntry
	var qferr error
	v := g.vimstate
	done := make(chan struct{})
	// Note at this point we can't schedule something in Vim because Vim is most likely
	// blocked waiting on an external command, e.g. fzf. So instead we enqueue a call
	g.Enqueue(func(_g govim.Govim) error {
		defer func() {
			if recover() == nil {
				close(done)
			}
		}()
		for _, si := range symbolResp {
			qf, err := v.locationToQuickfix(si.Location, *g.fRel)
			if err != nil {
				qferr = err
				return nil
			}
			qf.Text = si.Name
			qfs = append(qfs, qf)
		}
		return nil
	})
	<-done

	if qferr != nil {
		g.Errorf("failed to convert locations to quickfix entires: %v", err)
	}

	if *g.fQuickfix {
		for _, q := range qfs {
			g.PrintfStdout("%v:%v:%v: %v\n", q.Filename, q.Lnum, q.Col, q.Text)
		}
	}
}
