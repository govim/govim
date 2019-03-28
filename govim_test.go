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
	"github.com/myitcv/govim/internal/plugin"
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
				d := &driver{Driver: new(plugin.Driver)}
				td, err := testdriver.NewTestDriver(filepath.Base(e.WorkDir), e, errCh, d)
				if err != nil {
					t.Fatalf("failed to create new driver: %v", err)
				}
				if err := td.Run(); err != nil {
					td.Close()
					wg.Done()
					return err
				}
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
	*plugin.Driver
	driver *testdriver.TestDriver
}

func (d *driver) Init(g *govim.Govim) (err error) {
	d.Govim = g
	d.DefineFunction("Hello", []string{}, d.hello)
	d.DefineFunction("Bad", []string{}, d.bad)
	d.DefineRangeFunction("Echo", []string{}, d.echo)
	d.DefineCommand("HelloComm", d.helloComm, govim.AttrBang)
	d.DefineAutoCommand("", govim.Events{govim.EventBufRead}, govim.Patterns{"*.go"}, false, d.bufRead)
	return nil
}

func (d *driver) Shutdown() error {
	return nil
}

func (d *driver) bufRead() error {
	d.ChannelEx(`echom "Hello from BufRead"`)
	return nil
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
		line := d.ParseString(d.ChannelExprf("getline(%v)", i))
		lines = append(lines, line)
	}
	d.ChannelExf("echom %v", strconv.Quote(strings.Join(lines, "\n")))
	return nil, nil
}
