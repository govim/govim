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
	"time"

	"github.com/kr/pty"
	"github.com/myitcv/govim"
	"github.com/myitcv/govim/testsetup"
	"github.com/rogpeppe/go-internal/semver"
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

func NewTestDriver(name string, govimPath, testHomePath, testPluginPath string, env *testscript.Env, plug govim.Plugin) (*TestDriver, error) {
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

	if err := copyDir(testPluginPath, govimPath); err != nil {
		return nil, fmt.Errorf("failed to copy %v to %v: %v", govimPath, testPluginPath, err)
	}
	srcVimrc := filepath.Join(govimPath, "cmd", "govim", "config", "minimal.vimrc")
	dstVimrc := filepath.Join(testHomePath, ".vimrc")
	if err := copyFile(dstVimrc, srcVimrc); err != nil {
		return nil, fmt.Errorf("failed to copy %v to %v: %v", srcVimrc, dstVimrc, err)
	}

	res.govimListener = gl
	res.driverListener = dl

	env.Vars = append(env.Vars,
		"GOVIMTEST_SOCKET="+res.govimListener.Addr().String(),
		"GOVIMTESTDRIVER_SOCKET="+res.driverListener.Addr().String(),
	)

	_, cmd, err := testsetup.EnvLookupFlavorCommand()
	if err != nil {
		return nil, err
	}

	vimCmd := cmd
	if e := os.Getenv("VIM_COMMAND"); e != "" {
		vimCmd = strings.Fields(e)
	}

	res.cmd = exec.Command(vimCmd[0], vimCmd[1:]...)
	res.cmd.Env = env.Vars
	res.cmd.Dir = env.WorkDir

	return res, nil
}

func copyDir(dst, src string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		switch path {
		case filepath.Join(src, ".git"), filepath.Join(src, "cmd", "govim", ".bin"):
			return filepath.SkipDir
		}
		rel := strings.TrimPrefix(path, src)
		if strings.HasPrefix(rel, string(os.PathSeparator)) {
			rel = strings.TrimPrefix(rel, string(os.PathSeparator))
		}
		dstpath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dstpath, 0777)
		}
		return copyFile(dstpath, path)
	})
}

func copyFile(dst, src string) error {
	r, err := os.Open(src)
	if err != nil {
		return err
	}
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	r.Close()
	return w.Close()
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
	}
	select {
	case <-d.doneQuitGovim:
	default:
		close(d.quitGovim)
	}
	select {
	case <-d.doneQuitDriver:
	default:
		close(d.quitDriver)
	}
	select {
	case <-d.doneQuitVim:
	default:
		func() {
			defer func() {
				if r := recover(); r != nil && r != govim.ErrShuttingDown {
					panic(r)
				}
			}()
			d.govim.ChannelEx("qall!")
		}()
		<-d.doneQuitVim
	}
	select {
	case <-d.doneQuitGovim:
	default:
		d.govimListener.Close()
		<-d.doneQuitGovim
	}
	select {
	case <-d.doneQuitDriver:
	default:
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
	defer close(d.doneQuitGovim)
	if err := d.govim.Run(); err != nil {
		select {
		case <-d.quitGovim:
		default:
			return fmt.Errorf("govim Run failed: %v", err)
		}
	}
	return nil
}

func (d *TestDriver) listenDriver() error {
	defer close(d.doneQuitDriver)
	err := d.govim.DoProto(func() error {
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
		return nil
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
	indent := fs.Bool("indent", false, "pretty indent resulting JSON")
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
		if *indent {
			enc.SetIndent("", "  ")
		}
		if err := enc.Encode(vimResp[1]); err != nil {
			ef("failed to format output of JSON: %v", err)
		}
	}
	conn.Close()
	return 0
}

// Sleep is a convenience function for those odd occasions when you
// need to drop in a sleep, e.g. waiting for CursorHold to trigger
func Sleep(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("sleep does not support neg")
	}
	if len(args) != 1 {
		ts.Fatalf("sleep expects a single argument; got %v", len(args))
	}
	d, err := time.ParseDuration(args[0])
	if err != nil {
		ts.Fatalf("failed to parse duration %q: %v", args[0], err)
	}
	time.Sleep(d)
}

func Condition(cond string) (bool, error) {
	envf, cmd, err := testsetup.EnvLookupFlavorCommand()
	if err != nil {
		return false, err
	}
	var f govim.Flavor
	switch {
	case strings.HasPrefix(cond, govim.FlavorVim.String()):
		f = govim.FlavorVim
	case strings.HasPrefix(cond, govim.FlavorGvim.String()):
		f = govim.FlavorGvim
	default:
		return false, fmt.Errorf("unknown condition %v", cond)
	}
	v := strings.TrimPrefix(cond, f.String())
	if envf != f {
		return false, nil
	}
	if v == "" {
		return true, nil
	}
	if v[0] != ':' {
		return false, fmt.Errorf("failed to find version separator")
	}
	v = v[1:]
	if !semver.IsValid(v) {
		return false, fmt.Errorf("%v is not a valid semver version", v)
	}
	switch f {
	case govim.FlavorVim, govim.FlavorGvim:
		cmd := cmd.BuildCommand("-v", "--version")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return false, fmt.Errorf("failed to run %v: %v\n%s", strings.Join(cmd.Args, " "), err, out)
		}
		version, err := govim.ParseVimVersion(out)
		if err != nil {
			return false, err
		}
		return semver.Compare(version, v) >= 0, nil
	}

	panic("should not reach here")
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
