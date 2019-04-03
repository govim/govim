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

	"github.com/kr/pty"
	"github.com/myitcv/govim"
	"github.com/rogpeppe/go-internal/testscript"
)

// TODO - this code is a mess and needs to be fixed

type TestDriver struct {
	govimListener  net.Listener
	driverListener net.Listener
	govim          govim.Govim

	cmd *exec.Cmd

	name string

	plugin signallingPlugin

	quitVim    chan bool
	quitGovim  chan bool
	quitDriver chan bool

	doneQuitVim    chan bool
	doneQuitGovim  chan bool
	doneQuitDriver chan bool

	errCh chan error
}

func NewTestDriver(name string, env *testscript.Env, errCh chan error, plug govim.Plugin) (*TestDriver, error) {
	res := &TestDriver{
		quitVim:    make(chan bool),
		quitGovim:  make(chan bool),
		quitDriver: make(chan bool),

		doneQuitVim:    make(chan bool),
		doneQuitGovim:  make(chan bool),
		doneQuitDriver: make(chan bool),

		name: name,

		plugin: newSignallingPlugin(plug),

		errCh: errCh,
	}
	gl, err := net.Listen("tcp4", "localhost:0")
	if err != nil {
		res.errorf("failed to create listener for govim: %v", err)
	}
	dl, err := net.Listen("tcp4", ":0")
	if err != nil {
		res.errorf("failed to create listener for driver: %v", err)
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

	res.cmd = exec.Command("vim", "-u", vimrc)
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

func (d *TestDriver) Run() (err error) {
	go d.runVim()
	err = d.listenGovim()
	<-d.plugin.initDone
	return
}

func (d *TestDriver) runVim() {
	thepty, err := pty.Start(d.cmd)
	if err != nil {
		d.errorf("failed to start %v: %v", strings.Join(d.cmd.Args, " "), err)
	}
	go func() {
		if err := d.cmd.Wait(); err != nil {
			select {
			case <-d.quitVim:
			default:
				d.errorf("vim exited: %v", err)
			}
		}
		thepty.Close()
		close(d.doneQuitVim)
	}()
	io.Copy(ioutil.Discard, thepty)
}

func (d *TestDriver) Close() {
	close(d.quitVim)
	d.cmd.Process.Kill()
	<-d.doneQuitVim
	close(d.quitGovim)
	close(d.quitDriver)
	d.govimListener.Close()
	d.driverListener.Close()
	<-d.doneQuitGovim
	<-d.doneQuitDriver
}

func (d *TestDriver) errorf(format string, args ...interface{}) {
	err := fmt.Errorf(d.name+": "+format, args...)
	fmt.Println(err)
	d.errCh <- err
}

func (d *TestDriver) listenGovim() error {
	conn, err := d.govimListener.Accept()
	if err != nil {
		return fmt.Errorf("failed to accept connection on %v: %v", d.govimListener.Addr(), err)
	}
	g, err := govim.NewGovim(d.plugin, conn, conn, ioutil.Discard)
	if err != nil {
		return fmt.Errorf("failed to create govim: %v", err)
	}
	d.govim = g

	go d.listenDriver()
	go d.runGovim()

	return nil
}

func (d *TestDriver) runGovim() {
	if err := d.govim.Run(); err != nil {
		select {
		case <-d.quitGovim:
		default:
			d.errorf("govim Run failed: %v", err)
		}
	}
	close(d.doneQuitGovim)
}

func (d *TestDriver) listenDriver() {
	err := d.govim.DoProto(func() {
	Accept:
		for {
			conn, err := d.driverListener.Accept()
			if err != nil {
				select {
				case <-d.quitDriver:
					break Accept
				default:
					d.errorf("failed to accept connection to driver on %v: %v", d.driverListener.Addr(), err)
				}
			}
			dec := json.NewDecoder(conn)
			var args []interface{}
			if err := dec.Decode(&args); err != nil {
				d.errorf("failed to read command for driver: %v", err)
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
				add(d.govim.ChannelRedraw(force == "force"))
			case "ex":
				expr := args[1].(string)
				add(d.govim.ChannelEx(expr))
			case "normal":
				expr := args[1].(string)
				add(d.govim.ChannelNormal(expr))
			case "expr":
				expr := args[1].(string)
				resp, err := d.govim.ChannelExpr(expr)
				add(err, resp)
			case "call":
				fn := args[1].(string)
				resp, err := d.govim.ChannelCall(fn, args[2:]...)
				add(err, resp)
			default:
				d.errorf("don't yet know how to handle %v", cmd)
			}
			enc := json.NewEncoder(conn)
			if err := enc.Encode(res); err != nil {
				d.errorf("failed to encode response %v: %v", res, err)
			}
			conn.Close()
		}
	})

	if err != nil {
		d.errorf("%v", err)
	}
	close(d.doneQuitDriver)
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
		uq, err := strconv.Unquote("\"" + a + "\"")
		if err != nil {
			ef("failed to unquote %q: %v", a, err)
		}
		if i <= 1 {
			jsonArgs = append(jsonArgs, strconv.Quote(uq))
		} else {
			jsonArgs = append(jsonArgs, uq)
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

func (s signallingPlugin) Init(d govim.Govim) error {
	defer close(s.initDone)
	return s.u.Init(d)
}

func (s signallingPlugin) Shutdown() error {
	return s.u.Shutdown()
}
