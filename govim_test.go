// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package govim_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/govim/govim"
	"github.com/govim/govim/internal/plugin"
	"github.com/govim/govim/testdriver"
	"github.com/govim/govim/testsetup"
	"github.com/rogpeppe/go-internal/testscript"
)

var (
	fDebugLog = flag.Bool("debugLog", false, "whether to log debugging info from vim, govim and the test shim")
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"vim": testdriver.Vim,
	}))
}

func TestScripts(t *testing.T) {
	t.Parallel()
	var workdir string
	if envworkdir := os.Getenv(testsetup.EnvTestscriptWorkdirRoot); envworkdir != "" {
		workdir = filepath.Join(envworkdir, "govim"+testsetup.RaceOrNot())
		os.MkdirAll(workdir, 0777)
	}

	var waitLock sync.Mutex
	var waitList []func() error

	t.Run("scripts", func(t *testing.T) {
		t.Parallel()
		testscript.Run(t, testscript.Params{
			WorkdirRoot: workdir,
			Dir:         "testdata",
			Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
				"sleep":       testdriver.Sleep,
				"errlogmatch": testdriver.ErrLogMatch,
			},
			Condition: testdriver.Condition,
			Setup: func(e *testscript.Env) error {
				// Set a special temp dir to make identifying it easier for log
				// scraping
				tmp := filepath.Join(e.WorkDir, "_tmp")
				if err := os.MkdirAll(tmp, 0777); err != nil {
					return fmt.Errorf("failed to create temp dir %v: %v", tmp, err)
				}
				home := filepath.Join(e.WorkDir, ".home")
				e.Vars = append(e.Vars,
					testsetup.EnvDisableUserBusy+"=true",
					"HOME="+home,
					"TMPDIR="+tmp,
				)
				if workdir != "" {
					e.Vars = append(e.Vars, "GOVIM_LOGFILE_TMPL=%v")
				}
				testPluginPath := filepath.Join(home, ".vim", "pack", "plugins", "start", "govim")

				var vimDebugLogPath, govimDebugLogPath string

				errLog := new(testdriver.LockingBuffer)
				outputs := []io.Writer{
					errLog,
				}
				e.Values[testdriver.KeyErrLog] = errLog
				if os.Getenv(testsetup.EnvTestscriptStderr) == "true" {
					outputs = append(outputs, os.Stderr)
				}

				if *fDebugLog {
					fmt.Printf("Vim home path is at %s\n", home)

					vimDebugLog, err := ioutil.TempFile("", "govim_test_script_vim_debug_log*")
					if err != nil {
						return fmt.Errorf("failed to create govim log file: %v", err)
					}
					vimDebugLogPath = vimDebugLog.Name()
					fmt.Printf("Vim debug logging enabled for %v at %v\n", filepath.Base(e.WorkDir), vimDebugLog.Name())
					tf, err := ioutil.TempFile("", "govim_test_script_govim_log*")
					if err != nil {
						return fmt.Errorf("failed to create govim log file: %v", err)
					}
					outputs = append(outputs, tf)
					govimDebugLogPath = tf.Name()
					fmt.Printf("logging %v to %v\n", filepath.Base(e.WorkDir), tf.Name())
				}
				d := newTestPlugin(plugin.NewDriver(""))

				config := &testdriver.Config{
					Name:           filepath.Base(e.WorkDir),
					GovimPath:      ".",
					TestHomePath:   home,
					TestPluginPath: testPluginPath,
					Env:            e,
					Plugin:         d,
					Log:            io.MultiWriter(outputs...),
					Debug: testdriver.Debug{
						Enabled: *fDebugLog,
						// FYI increasing this to 8 or above seems to cause Vim to do something weird with stdout, which means some tests fail
						VimLogLevel:  7,
						VimLogPath:   vimDebugLogPath,
						GovimLogPath: govimDebugLogPath,
					},
				}

				td, err := testdriver.NewTestDriver(config)
				if err != nil {
					return fmt.Errorf("failed to create new driver: %v", err)
				}

				if err := td.Run(); err != nil {
					return fmt.Errorf("failed to run TestDriver: %v", err)
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
	t.ChannelExf(`silent echom "Hello from BufRead %v"`, fn)
	return nil
}

func (t *testpluginvim) helloComm(flags govim.CommandFlags, args ...string) error {
	t.ChannelExf(`silent echom "Hello world (%v)"`, *flags.Bang)
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
