// Package testdriver is a support package for plugins written using github.com/myitcv/govim
package testdriver

import (
	"bytes"
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

	"github.com/kr/pty"
	"github.com/myitcv/govim"
	"github.com/rogpeppe/go-internal/testscript"
	"gopkg.in/tomb.v2"
)

// TODO - this code is a mess and needs to be fixed

type TestDriver struct {
	govimListener  net.Listener
	driverListener net.Listener
	govim          govim.Govim

	Log io.Writer

	cmd *exec.Cmd

	name string

	plugin signallingPlugin

	quitVim    chan bool
	quitGovim  chan bool
	quitDriver chan bool

	doneQuitVim    chan bool
	doneQuitGovim  chan bool
	doneQuitDriver chan bool

	tomb tomb.Tomb

	closeLock sync.Mutex
	closed    bool
}

func NewTestDriver(name string, env *testscript.Env, plug govim.Plugin) (*TestDriver, error) {
	res := &TestDriver{
		quitVim:    make(chan bool),
		quitGovim:  make(chan bool),
		quitDriver: make(chan bool),

		doneQuitVim:    make(chan bool),
		doneQuitGovim:  make(chan bool),
		doneQuitDriver: make(chan bool),

		name: name,

		plugin: newSignallingPlugin(plug),
	}
	gl, err := net.Listen("tcp4", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener for govim: %v", err)
	}
	dl, err := net.Listen("tcp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener for driver: %v", err)
	}

	res.govimListener = gl
	res.driverListener = dl

	env.Vars = append(env.Vars,
		"GOVIMTEST_SOCKET="+res.govimListener.Addr().String(),
		"GOVIMTESTDRIVER_SOCKET="+res.driverListener.Addr().String(),
	)

	vimrc, err := findLocalVimrc()
	if err != nil {
		return nil, fmt.Errorf("failed to find local vimrc: %v", err)
	}

	vimCmd := []string{"vim"}
	if e := os.Getenv("VIM_COMMAND"); e != "" {
		vimCmd = strings.Fields(e)
	}
	vimCmd = append(vimCmd, "-u", vimrc)

	res.cmd = exec.Command(vimCmd[0], vimCmd[1:]...)
	res.cmd.Env = env.Vars

	for i := len(env.Vars) - 1; i >= 0; i-- {
		if strings.HasPrefix(env.Vars[i], "WORK=") {
			res.cmd.Dir = strings.TrimPrefix(env.Vars[i], "WORK=")
			break
		}
	}

	return res, nil
}

func findLocalVimrc() (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "list", "-f={{.Dir}}", "github.com/myitcv/govim/testdriver")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run %v: %v\n%s", strings.Join(cmd.Args, " "), err, stderr.Bytes())
	}
	dir := strings.TrimSpace(stdout.String())
	return filepath.Join(dir, "test.vim"), nil
}

func (d *TestDriver) Run() {
	d.tombgo(d.runVim)
	d.tombgo(d.listenGovim)
	select {
	case <-d.tomb.Dying():
	case <-d.plugin.initDone:
	}
}

func (d *TestDriver) Wait() error {
	return d.tomb.Wait()
}

func (d *TestDriver) runVim() error {
	thepty, err := pty.Start(d.cmd)
	if err != nil {
		close(d.doneQuitVim)
		return fmt.Errorf("failed to start %v: %v", strings.Join(d.cmd.Args, " "), err)
	}
	d.tombgo(func() error {
		defer func() {
			thepty.Close()
			close(d.doneQuitVim)
		}()
		if err := d.cmd.Wait(); err != nil {
			select {
			case <-d.quitVim:
			default:
				return fmt.Errorf("vim exited: %v", err)
			}
		}
		return nil
	})
	io.Copy(ioutil.Discard, thepty)
	return nil
}

func (d *TestDriver) Close() {
	d.closeLock.Lock()
	if d.closed {
		d.closeLock.Unlock()
		return
	}
	d.closed = true
	d.closeLock.Unlock()
	select {
	case <-d.doneQuitVim:
	default:
		close(d.quitVim)
		d.cmd.Process.Kill()
		<-d.doneQuitVim
	}
	select {
	case <-d.doneQuitGovim:
	default:
		close(d.quitGovim)
		d.govimListener.Close()
		<-d.doneQuitGovim
	}
	select {
	case <-d.doneQuitDriver:
	default:
		close(d.quitDriver)
		d.driverListener.Close()
		<-d.doneQuitDriver
	}
}

func (d *TestDriver) tombgo(f func() error) {
	d.tomb.Go(func() error {
		err := f()
		if err != nil {
			fmt.Printf(">>> %v\n", err)
			d.Close()
		}
		return err
	})
}

func (d *TestDriver) listenGovim() error {
	good := false
	defer func() {
		if !good {
			close(d.doneQuitGovim)
			close(d.doneQuitDriver)
		}
	}()
	conn, err := d.govimListener.Accept()
	if err != nil {
		select {
		case <-d.quitGovim:
			return nil
		default:
			return fmt.Errorf("failed to accept connection on %v: %v", d.govimListener.Addr(), err)
		}
	}
	var log io.Writer = ioutil.Discard
	if d.Log != nil {
		log = d.Log
	}
	g, err := govim.NewGovim(d.plugin, conn, conn, log)
	if err != nil {
		return fmt.Errorf("failed to create govim: %v", err)
	}
	good = true
	d.govim = g
	d.tombgo(d.listenDriver)
	d.tombgo(d.runGovim)

	return nil
}

func (d *TestDriver) runGovim() error {
	if err := d.govim.Run(); err != nil {
		select {
		case <-d.quitGovim:
		default:
			return fmt.Errorf("govim Run failed: %v", err)
		}
	}
	close(d.doneQuitGovim)
	return nil
}

func (d *TestDriver) listenDriver() error {
	defer close(d.doneQuitDriver)
	err := d.govim.DoProto(func() {
	Accept:
		for {
			conn, err := d.driverListener.Accept()
			if err != nil {
				select {
				case <-d.quitDriver:
					break Accept
				default:
					panic(fmt.Errorf("failed to accept connection to driver on %v: %v", d.driverListener.Addr(), err))
				}
			}
			dec := json.NewDecoder(conn)
			var args []interface{}
			if err := dec.Decode(&args); err != nil {
				panic(fmt.Errorf("failed to read command for driver: %v", err))
			}
			cmd := args[0]
			res := []interface{}{""}
			add := func(err error, is ...interface{}) {
				toAdd := []interface{}{""}
				if err != nil {
					toAdd[0] = err.Error()
				} else {
					toAdd = append(toAdd, is...)
				}
				res = append(res, toAdd)
			}
			switch cmd {
			case "redraw":
				var force string
				if len(args) == 2 {
					force = args[1].(string)
				}
				<-d.govim.Schedule(func(g govim.Govim) error {
					add(g.ChannelRedraw(force == "force"))
					return nil
				})
			case "ex":
				expr := args[1].(string)
				<-d.govim.Schedule(func(g govim.Govim) error {
					add(g.ChannelEx(expr))
					return nil
				})
			case "normal":
				expr := args[1].(string)
				<-d.govim.Schedule(func(g govim.Govim) error {
					add(g.ChannelNormal(expr))
					return nil
				})
			case "expr":
				expr := args[1].(string)
				<-d.govim.Schedule(func(g govim.Govim) error {
					resp, err := g.ChannelExpr(expr)
					add(err, resp)
					return nil
				})
			case "call":
				fn := args[1].(string)
				<-d.govim.Schedule(func(g govim.Govim) error {
					resp, err := g.ChannelCall(fn, args[2:]...)
					add(err, resp)
					return nil
				})
			default:
				panic(fmt.Errorf("don't yet know how to handle %v", cmd))
			}
			enc := json.NewEncoder(conn)
			if err := enc.Encode(res); err != nil {
				panic(fmt.Errorf("failed to encode response %v: %v", res, err))
			}
			conn.Close()
		}
	})

	if err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}

// Vim is a sidecar that effectively drives Vim via a simple JSON-based
// API
func Vim() (exitCode int) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		exitCode = -1
		fmt.Fprintln(os.Stderr, r)
	}()
	ef := func(format string, args ...interface{}) {
		panic(fmt.Sprintf(format, args...))
	}
	fs := flag.NewFlagSet("vim", flag.PanicOnError)
	bang := fs.Bool("bang", false, "expect command to fail")
	fs.Parse(os.Args[1:])
	args := fs.Args()
	fn := args[0]
	var jsonArgs []string
	for i, a := range args {
		if i <= 1 {
			uq, err := strconv.Unquote("\"" + a + "\"")
			if err != nil {
				ef("failed to unquote %q: %v", a, err)
			}
			jsonArgs = append(jsonArgs, strconv.Quote(uq))
		} else {
			var buf bytes.Buffer
			json.HTMLEscape(&buf, []byte(a))
			jsonArgs = append(jsonArgs, buf.String())
		}
	}
	jsonArgString := "[" + strings.Join(jsonArgs, ", ") + "]"
	var i []interface{}
	if err := json.Unmarshal([]byte(jsonArgString), &i); err != nil {
		ef("failed to json Unmarshal %q: %v", jsonArgString, err)
	}
	switch fn {
	case "redraw":
		// optional argument of force
		switch l := len(args[1:]); l {
		case 0:
		case 1:
			if args[1] != "force" {
				ef("unknown argument %q to redraw", args[1])
			}
		default:
			ef("redraw has a single optional argument: force; we saw %v", l)
		}
	case "ex", "normal", "expr":
		switch l := len(args[1:]); l {
		case 1:
			if _, ok := i[1].(string); !ok {
				ef("%v takes a string argument; saw %T", fn, i[1])
			}
		default:
			ef("%v takes a single argument: we saw %v", fn, l)
		}
	case "call":
		switch l := len(args[1:]); l {
		case 1:
			// no args
			if _, ok := i[1].(string); !ok {
				ef("%v takes a string as its first argument; saw %T", fn, i[1])
			}
		case 2:
			if _, ok := i[1].(string); !ok {
				ef("%v takes a string as its first argument; saw %T", fn, i[1])
			}
			vs, ok := i[2].([]interface{})
			if !ok {
				ef("%v takes a slice of values as its second argument; saw %T", fn, i[2])
			}
			// on the command line we require the args to be specified as an array
			// for ease of explanation/documentation, but now we splat the slice
			i = append(i[:2], vs...)
		default:
			ef("%v takes a two arguments: we saw %v", fn, l)
		}
	}
	if bs, err := json.Marshal(i); err != nil {
		ef("failed to remarshal json args: %v", err)
	} else {
		jsonArgString = string(bs)
	}
	addr := os.Getenv("GOVIMTESTDRIVER_SOCKET")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		ef("failed to connect to driver on %v: %v", addr, err)
	}
	if _, err := fmt.Fprintln(conn, jsonArgString); err != nil {
		ef("failed to send command %q to driver on: %v", jsonArgString, err)
	}
	dec := json.NewDecoder(conn)
	var resp []interface{}
	if err := dec.Decode(&resp); err != nil {
		ef("failed to decode response: %v", err)
	}
	if resp[0] != "" {
		// this is a protocol-level error
		ef("got error response: %v", resp[0])
	}
	// resp[1] will be a []intferface{} where the first
	// element will be a Vim-level error
	vimResp := resp[1].([]interface{})
	if err := vimResp[0].(string); err != "" {
		// this was a vim-level error
		if !*bang {
			ef("unexpected command error: %v", err)
		}
		fmt.Fprintln(os.Stderr, err)
	}
	if len(vimResp) == 2 {
		if *bang {
			ef("unexpected command success")
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(vimResp[1]); err != nil {
			ef("failed to format output of JSON: %v", err)
		}
	}
	conn.Close()
	return 0
}

type signallingPlugin struct {
	u        govim.Plugin
	initDone chan bool
}

func newSignallingPlugin(g govim.Plugin) signallingPlugin {
	return signallingPlugin{
		u:        g,
		initDone: make(chan bool),
	}
}

func (s signallingPlugin) Init(d govim.Govim, errCh chan error) error {
	defer close(s.initDone)
	return s.u.Init(d, errCh)
}

func (s signallingPlugin) Shutdown() error {
	return s.u.Shutdown()
}
