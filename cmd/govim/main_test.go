// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/kr/pty"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"vim": vim,
	}))
}

func TestScripts(t *testing.T) {
	var wg sync.WaitGroup
	errCh := make(chan error)

	t.Run("scripts", func(t *testing.T) {
		testscript.Run(t, testscript.Params{
			Dir: "testdata",
			Setup: func(e *testscript.Env) error {
				wg.Add(1)
				wd, err := os.Getwd()
				if err != nil {
					t.Fatalf("failed to get working directory: %v", err)
				}
				d, err := newDriver(filepath.Join(wd, "run.vim"), e, errCh)
				if err != nil {
					t.Fatalf("failed to create new driver: %v", err)
				}
				d.run()
				e.Defer(func() {
					d.close()
					wg.Done()
				})
				return nil
			},
		})
	})

	go func() {
		wg.Wait()
		close(errCh)
	}()

	if err, ok := <-errCh; ok {
		t.Fatal(err)
	}
}

type driver struct {
	govimListener  net.Listener
	driverListener net.Listener
	govim          *govim

	cmd *exec.Cmd

	quitVim    chan bool
	quitGovim  chan bool
	quitDriver chan bool

	doneQuitVim    chan bool
	doneQuitGovim  chan bool
	doneQuitDriver chan bool

	errCh chan error
}

func newDriver(vimrc string, env *testscript.Env, errCh chan error) (*driver, error) {
	res := &driver{
		quitVim:    make(chan bool),
		quitGovim:  make(chan bool),
		quitDriver: make(chan bool),

		doneQuitVim:    make(chan bool),
		doneQuitGovim:  make(chan bool),
		doneQuitDriver: make(chan bool),

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

func (d *driver) run() {
	go d.listenGovim()
	if _, err := pty.Start(d.cmd); err != nil {
		d.errorf("failed to start %v: %v", strings.Join(d.cmd.Args, " "), err)
	}
	go func() {
		if err := d.cmd.Wait(); err != nil {
			select {
			case <-d.quitVim:
				close(d.doneQuitVim)
			default:
				d.errorf("vim exited: %v", err)
			}
		}
	}()
}

func (d *driver) close() {
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

func (d *driver) errorf(format string, args ...interface{}) {
	err := fmt.Errorf(format, args...)
	panic(err)
	fmt.Printf("%v\n", err)
	d.errCh <- fmt.Errorf(format, args...)
}

func (d *driver) listenGovim() {
	conn, err := d.govimListener.Accept()
	if err != nil {
		d.errorf("failed to accept connection on %v: %v", d.govimListener.Addr(), err)
	}
	g, err := newGoVim(conn, conn)
	if err != nil {
		d.errorf("failed to create govim: %v", err)
	}
	d.govim = g

	go d.listenDriver()

	if err := g.Run(); err != nil {
		select {
		case <-d.quitGovim:
		default:
			d.errorf("govim Run failed: %v", err)
		}
	}
	close(d.doneQuitGovim)
}

func (d *driver) listenDriver() {
	err := d.govim.doProto(func() {
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
			switch cmd {
			case "redraw":
				var force string
				if len(args) == 2 {
					force = args[1].(string)
				}
				if err := d.govim.ChannelRedraw(force == "force"); err != nil {
					d.errorf("failed to execute %v: %v", cmd, err)
				}
			case "ex":
				expr := args[1].(string)
				if err := d.govim.ChannelEx(expr); err != nil {
					d.errorf("failed to ChannelEx %v: %v", cmd, err)
				}
			case "normal":
				expr := args[1].(string)
				if err := d.govim.ChannelNormal(expr); err != nil {
					d.errorf("failed to ChannelNormal %v: %v", cmd, err)
				}
			case "expr":
				expr := args[1].(string)
				resp, err := d.govim.ChannelExpr(expr)
				if err != nil {
					d.errorf("failed to ChannelExpr %v: %v", cmd, err)
				}
				res = append(res, resp)
			case "call":
				fn := args[1].(string)
				resp, err := d.govim.ChannelCall(fn, args[2:]...)
				if err != nil {
					d.errorf("failed to ChannelCall %v: %v", cmd, err)
				}
				res = append(res, resp)
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

// vim is a sidecar that effectively drives
func vim() (exitCode int) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		exitCode = -1
		panic(r)
		fmt.Fprintln(os.Stderr, r)
	}()
	ef := func(format string, args ...interface{}) {
		panic(fmt.Sprintf(format, args...))
	}
	args := os.Args[1:]
	fn := args[0]
	var jsonArgs []string
	for i, a := range args {
		if i <= 1 {
			jsonArgs = append(jsonArgs, strconv.Quote(a))
		} else {
			jsonArgs = append(jsonArgs, a)
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
		case 2:
			if _, ok := i[1].(string); !ok {
				ef("%v takes a string as its first argument; saw %T", fn, i[1])
			}
			if _, ok := i[2].([]interface{}); !ok {
				ef("%v takes a slice of values as its second argument; saw %T", fn, i[2])
			}
		default:
			ef("%v takes a two arguments: we saw %v", fn, l)
		}
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
		ef("got error response: %v", resp[0])
	}
	if len(resp) == 2 {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(resp[1]); err != nil {
			ef("failed to format output of JSON: %v", err)
		}
	}
	conn.Close()
	return 0
}
