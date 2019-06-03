// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package govim_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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

var (
	fLogGovim = flag.Bool("govimLog", false, "whether to log govim activity")
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"vim": testdriver.Vim,
	}))
}

func TestScripts(t *testing.T) {
	var waitLock sync.Mutex
	var waitList []func() error

	t.Run("scripts", func(t *testing.T) {
		testscript.Run(t, testscript.Params{
			Dir: "testdata",
			Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
				"sleep": testdriver.Sleep,
			},
			Condition: testdriver.Condition,
			Setup: func(e *testscript.Env) error {
				home := filepath.Join(e.WorkDir, "home")
				e.Vars = append(e.Vars,
					"HOME="+home,
				)
				testPluginPath := filepath.Join(e.WorkDir, "home", ".vim", "pack", "plugins", "start", "govim")
				d := newTestPlugin(plugin.NewDriver(""))
				td, err := testdriver.NewTestDriver(filepath.Base(e.WorkDir), ".", home, testPluginPath, e, d)
				if err != nil {
					t.Fatalf("failed to create new driver: %v", err)
				}
				if *fLogGovim {
					tf, err := ioutil.TempFile("", "govim_test_script_govim_log*")
					if err != nil {
						t.Fatalf("failed to create govim log file: %v", err)
					}
					td.Log = tf
					t.Logf("logging %v to %v\n", filepath.Base(e.WorkDir), tf.Name())
				}
				if err := td.Run(); err != nil {
					t.Fatalf("failed to run TestDriver: %v", err)
				}
				waitLock.Lock()
				waitList = append(waitList, td.Wait)
				waitLock.Unlock()
				e.Defer(func() {
					td.Close()
				})
				return nil
			},
		})
	})

	var errLock sync.Mutex
	var errors []error

	var wg sync.WaitGroup

	for _, w := range waitList {
		w := w
		wg.Add(1)
		go func() {
			if err := w(); err != nil {
				errLock.Lock()
				errors = append(errors, err)
				errLock.Unlock()
			}
			wg.Done()
		}()
	}

	wg.Wait()

	if len(errors) > 0 {
		var msgs []string
		for _, e := range errors {
			msgs = append(msgs, e.Error())
		}
		t.Fatalf("got some errors:\n%v\n", strings.Join(msgs, "\n"))
	}
}

type testplugin struct {
	plugin.Driver
	*testpluginvim
}

type testpluginvim struct {
	plugin.Driver
	*testplugin
}

func newTestPlugin(d plugin.Driver) *testplugin {
	res := &testplugin{
		Driver: d,
		testpluginvim: &testpluginvim{
			Driver: d,
		},
	}
	res.testpluginvim.testplugin = res
	return res
}

func (t *testplugin) Init(g govim.Govim, errCh chan error) (err error) {
	t.Driver.Govim = g
	t.testpluginvim.Driver.Govim = g.Scheduled()
	t.DefineFunction("HelloNil", nil, t.hello)
	t.DefineFunction("Hello", []string{}, t.hello)
	t.DefineFunction("HelloWithArg", []string{"target"}, t.helloWithArg)
	t.DefineFunction("HelloWithVarArgs", []string{"target", "..."}, t.helloWithVarArgs)
	t.DefineFunction("Bad", []string{}, t.bad)
	t.DefineRangeFunction("Echo", []string{}, t.echo)
	t.DefineCommand("HelloComm", t.helloComm, govim.AttrBang)
	t.DefineAutoCommand("", govim.Events{govim.EventBufRead}, govim.Patterns{"*.go"}, false, t.bufRead, "expand('<afile>')")
	t.DefineFunction("Func1", []string{}, t.func1)
	t.DefineFunction("Func2", []string{}, t.func2)
	t.DefineFunction("TriggerUnscheduled", []string{}, t.triggerUnscheduled)
	t.DefineFunction("VersionCheck", []string{}, t.versionCheck)
	return nil
}

func (t *testplugin) Shutdown() error {
	return nil
}

func (t *testpluginvim) bufRead(args ...json.RawMessage) error {
	// we are expecting the expanded result of <afile> as our first argument
	fn := t.ParseString(args[0])
	t.ChannelExf(`echom "Hello from BufRead %v"`, fn)
	return nil
}

func (t *testpluginvim) helloComm(flags govim.CommandFlags, args ...string) error {
	t.ChannelExf(`echom "Hello world (%v)"`, *flags.Bang)
	return nil
}

func (t *testpluginvim) hello(args ...json.RawMessage) (interface{}, error) {
	return "World", nil
}

func (t *testpluginvim) helloWithArg(args ...json.RawMessage) (interface{}, error) {
	// Params: (target string)
	return t.ParseString(args[0]), nil
}

func (t *testpluginvim) helloWithVarArgs(args ...json.RawMessage) (interface{}, error) {
	// Params: (target string, others ...string)
	parts := []string{t.ParseString(args[0])}
	varargs := t.ParseJSONArgSlice(args[1])
	for _, a := range varargs {
		parts = append(parts, t.ParseString(a))
	}
	return strings.Join(parts, " "), nil
}

func (t *testpluginvim) bad(args ...json.RawMessage) (interface{}, error) {
	return nil, fmt.Errorf("this is a bad function")
}

func (t *testpluginvim) echo(first, last int, jargs ...json.RawMessage) (interface{}, error) {
	args := make([]interface{}, len(jargs))
	for i, a := range jargs {
		if err := json.Unmarshal(a, &args[i]); err != nil {
			return nil, fmt.Errorf("failed to unmarshal arg %v: %v", i+1, err)
		}
	}
	var lines []string
	for i := first; i <= last; i++ {
		line := t.ParseString(t.ChannelExprf("getline(%v)", i))
		lines = append(lines, line)
	}
	t.ChannelExf("echom %v", strconv.Quote(strings.Join(lines, "\n")))
	return nil, nil
}

func (t *testpluginvim) func1(args ...json.RawMessage) (interface{}, error) {
	res := t.ParseString(t.ChannelCall("Func2"))
	return res, nil
}

func (t *testpluginvim) func2(args ...json.RawMessage) (interface{}, error) {
	return "World from Func2", nil
}

func (t *testpluginvim) triggerUnscheduled(args ...json.RawMessage) (interface{}, error) {
	go func() {
		t.testplugin.Schedule(func(g govim.Govim) error {
			g.ChannelNormal("iHello Gophers")
			g.ChannelEx("w out")
			return nil
		})
	}()
	return nil, nil
}

func (t *testpluginvim) versionCheck(args ...json.RawMessage) (interface{}, error) {
	return fmt.Sprintf("%v %v", t.Flavor(), t.Version()), nil
}
