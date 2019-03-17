// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package govim_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/testdriver"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"vim": testdriver.Vim,
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
				d := new(driver)
				td, err := testdriver.NewDriver(filepath.Base(e.WorkDir), e, errCh, d.init)
				if err != nil {
					t.Fatalf("failed to create new driver: %v", err)
				}
				td.Run()
				e.Defer(func() {
					td.Close()
					wg.Done()
				})
				return nil
			},
		})
	})

	errsDone := make(chan bool)

	var errs []error

	go func() {
		for err, ok := <-errCh; ok; {
			errs = append(errs, err)
		}
		close(errsDone)
	}()

	go func() {
		wg.Wait()
		close(errCh)
	}()

	<-errsDone

	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		t.Fatalf("got some errors:\n%v\n", strings.Join(msgs, "\n"))
	}
}

type driver struct {
	driver *testdriver.Driver
	*govim.Govim
}

func (d *driver) init(g *govim.Govim) error {
	d.Govim = g
	return d.do(func() error {
		d.DefineFunction("Hello", []string{}, d.hello)
		d.DefineFunction("Bad", []string{}, d.bad)
		d.DefineRangeFunction("Echo", []string{}, d.echo)
		d.DefineCommand("HelloComm", d.helloComm, govim.AttrBang)
		return nil
	})
}

func (d *driver) helloComm(flags govim.CommandFlags, args ...string) error {
	d.ChannelExf(`echom "Hello world (%v)"`, *flags.Bang)
	return nil
}

func (d *driver) hello(args ...json.RawMessage) (interface{}, error) {
	return "World", nil
}

func (d *driver) bad(args ...json.RawMessage) (interface{}, error) {
	return nil, fmt.Errorf("this is a bad function")
}

func (d *driver) echo(first, last int, jargs ...json.RawMessage) (interface{}, error) {
	args := make([]interface{}, len(jargs))
	for i, a := range jargs {
		if err := json.Unmarshal(a, &args[i]); err != nil {
			return nil, fmt.Errorf("failed to unmarshal arg %v: %v", i+1, err)
		}
	}
	var lines []string
	for i := first; i <= last; i++ {
		line := d.parseString(d.ChannelExprf("getline(%v)", i))
		lines = append(lines, line)
	}
	d.ChannelExf("echom %v", strconv.Quote(strings.Join(lines, "\n")))
	return nil, nil
}

func (d *driver) do(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r := r.(type) {
			case errDriver:
				err = r
			default:
				panic(r)
			}
		}
	}()
	return f()
}

func (d *driver) doFunction(f govim.VimFunction) govim.VimFunction {
	return func(args ...json.RawMessage) (interface{}, error) {
		var i interface{}
		err := d.do(func() error {
			var err error
			i, err = f(args...)
			return err
		})
		if err != nil {
			return nil, err
		}
		return i, nil
	}
}

func (d *driver) doRangeFunction(f govim.VimRangeFunction) govim.VimRangeFunction {
	return func(first, last int, args ...json.RawMessage) (interface{}, error) {
		var i interface{}
		err := d.do(func() error {
			var err error
			i, err = f(first, last, args...)
			return err
		})
		if err != nil {
			return nil, err
		}
		return i, nil
	}
}

func (d *driver) errorf(format string, args ...interface{}) {
	panic(errDriver{underlying: fmt.Errorf(format, args...)})
}

func (d *driver) ChannelExpr(expr string) json.RawMessage {
	i, err := d.Govim.ChannelExpr(expr)
	if err != nil {
		d.errorf("ChannelExpr(%q) failed: %v", expr, err)
	}
	return i
}

func (d *driver) ChannelEx(expr string) {
	if err := d.Govim.ChannelEx(expr); err != nil {
		d.errorf("ChannelEx(%q) failed: %v", expr, err)
	}
}

func (d *driver) parseString(j json.RawMessage) string {
	var v string
	if err := json.Unmarshal(j, &v); err != nil {
		d.errorf("failed to parse string from %q: %v", j, err)
	}
	return v
}

func (d *driver) parseInt(j json.RawMessage) int {
	var v int
	if err := json.Unmarshal(j, &v); err != nil {
		d.errorf("failed to parse int from %q: %v", j, err)
	}
	return v
}

func (d *driver) ChannelExprf(format string, args ...interface{}) json.RawMessage {
	return d.ChannelExpr(fmt.Sprintf(format, args...))
}

func (d *driver) ChannelExf(format string, args ...interface{}) {
	d.ChannelEx(fmt.Sprintf(format, args...))
}

func (d *driver) DefineFunction(name string, args []string, f govim.VimFunction) {
	if err := d.Govim.DefineFunction(name, args, d.doFunction(f)); err != nil {
		d.errorf("failed to DefineFunction %q: %v", name, err)
	}
}

func (d *driver) DefineRangeFunction(name string, args []string, f govim.VimRangeFunction) {
	if err := d.Govim.DefineRangeFunction(name, args, d.doRangeFunction(f)); err != nil {
		d.errorf("failed to DefineRangeFunction %q: %v", name, err)
	}
}

func (d *driver) DefineCommand(name string, f govim.VimCommandFunction, attrs ...govim.CommAttr) {
	if err := d.Govim.DefineCommand(name, f, attrs...); err != nil {
		d.errorf("failed to DefineCommand %q: %v", name, err)
	}
}

type errDriver struct {
	underlying error
}

func (e errDriver) Error() string {
	return fmt.Sprintf("driver error: %v", e.underlying)
}
