// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/creack/pty"
	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/testdriver"
	"github.com/govim/govim/testsetup"
	"github.com/rogpeppe/go-internal/goproxytest"
	"github.com/rogpeppe/go-internal/gotooltest"
	"github.com/rogpeppe/go-internal/testscript"
)

const (
	EnvInstallScripts = "GOVIM_RUN_INSTALL_TESTSCRIPTS"
)

var (
	fGoplsPath = flag.String("gopls", "", "Path to the gopls binary for use in scenario tests. If unset, gopls is built from a tagged version.")
)

func init() {
	exposeTestAPI = true
}

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"vim":     testdriver.Vim,
		"execvim": execvim,
	}))
}

func TestScripts(t *testing.T) {
	// TODO our approach with setting the workdir root via os.Setenv("GOTMPDIR")
	// is hacky and gross. Working out a cleaner approach with rogpeppe, likely
	// passing in such a value via Params
	var workdir string
	if envworkdir := os.Getenv(testsetup.EnvTestscriptWorkdirRoot); envworkdir == "" {
		// i.e. we are not going to call os.Setenv below
		t.Parallel()
	} else {
		workdir = filepath.Join(envworkdir, "cmd", "govim"+raceOrNot())
		defer os.Setenv("GOTMPDIR", os.Getenv("GOTMPDIR"))
	}

	var waitLock sync.Mutex
	var waitList []func() error

	goplsPath := *fGoplsPath
	if goplsPath == "" {
		td, err := installGoplsToTempDir()
		if err != nil {
			t.Fatalf("failed to install gopls to temp directory: %v", err)
		}
		defer os.RemoveAll(td)
		goplsPath = filepath.Join(td, "gopls")
	}
	t.Logf("using gopls at %q", goplsPath)
	govimPath := strings.TrimSpace(runCmd(t, "go", "list", "-m", "-f={{.Dir}}"))

	proxy, err := goproxytest.NewServer("testdata/mod", "")
	if err != nil {
		t.Fatalf("cannot start proxy: %v", err)
	}

	entries, err := ioutil.ReadDir("testdata")
	if err != nil {
		t.Fatalf("failed to list testdata: %v", err)
	}
	for _, entry := range entries {
		entry := entry
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "scenario_") {
			continue
		}
		workdir := workdir
		if workdir != "" {
			workdir = filepath.Join(workdir, entry.Name())
			os.MkdirAll(workdir, 0777)
			os.Setenv("GOTMPDIR", workdir)
		}
		t.Run(entry.Name(), func(t *testing.T) {
			// Do not call t.Parallel here. We currently rely on this being
			// run on the test goroutine in order that os.Setenv is not racey
			params := testscript.Params{
				TestWork: workdir != "",
				Dir:      filepath.Join("testdata", entry.Name()),
				Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
					"sleep":       testdriver.Sleep,
					"errlogmatch": testdriver.ErrLogMatch,
				},
				Condition: testdriver.Condition,
				Setup: func(e *testscript.Env) error {
					// We set a special TMPDIR so the file watcher ignores it
					tmp := filepath.Join(e.WorkDir, "_tmp")
					if err := os.MkdirAll(tmp, 0777); err != nil {
						return fmt.Errorf("failed to create temp dir %v: %v", tmp, err)
					}
					home := filepath.Join(e.WorkDir, ".home")
					e.Vars = append(e.Vars,
						"DEFAULT_ERRLOGMATCH_WAIT="+testdriver.DefaultErrLogMatchWait,
						"TMPDIR="+tmp,
						"GOPROXY="+proxy.URL,
						"GONOSUMDB=*",
						"HOME="+home,
						"PLUGIN_PATH="+govimPath,
					)
					if workdir != "" {
						e.Vars = append(e.Vars, "GOVIM_LOGFILE_TMPL=%v")
					}
					testPluginPath := filepath.Join(home, ".vim", "pack", "plugins", "start", "govim")

					errLog := new(testdriver.LockingBuffer)
					outputs := []io.Writer{
						errLog,
					}
					e.Values[testdriver.KeyErrLog] = errLog
					if os.Getenv(testsetup.EnvTestscriptStderr) == "true" {
						outputs = append(outputs, os.Stderr)
					}

					var err error
					var tf *os.File
					if workdir == "" {
						tf, err = ioutil.TempFile(tmp, "govim.log*")
						if err != nil {
							t.Fatalf("failed to create govim log file: %v", err)
						}
					} else {
						// create a "plain"-named logfile because as above we set
						// GOVIM_LOGFILE_TMPL=%v
						tf, err = os.OpenFile(filepath.Join(tmp, "govim.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
						if err != nil {
							t.Fatalf("failed to create non-tmp govim log file: %v", err)
						}
					}
					e.Defer(func() {
						if err := tf.Close(); err != nil {
							panic(fmt.Errorf("failed to close govim logfile %v: %v", tf.Name(), err))
						}
					})
					outputs = append(outputs, tf)

					defaultsPath := filepath.Join("testdata", entry.Name(), "default_config.json")
					defaults, err := readConfig(defaultsPath)
					if err != nil {
						t.Fatalf("failed to read defaults from %v: %v", defaultsPath, err)
					}
					// We now ensure we have a default for at least CompletionBudget because
					// unless we are specifically testing the behaviour of that config value
					// (which would be unusual because it's really only intended for integration
					// tests) we definitely want to set the value to "0ms"
					userPath := filepath.Join("testdata", entry.Name(), "user_config.json")
					user, err := readConfig(userPath)
					if err != nil {
						t.Fatalf("failed to read user from %v: %v", userPath, err)
					}
					if user == nil {
						user = new(config.Config)
					}
					if user.CompletionBudget == nil {
						s := "0ms"
						user.CompletionBudget = &s
					}

					d := newplugin(string(goplsPath), e.Vars, defaults, user)

					config := &testdriver.Config{
						Name:           filepath.Base(e.WorkDir),
						GovimPath:      govimPath,
						ReadLog:        errLog,
						Log:            io.MultiWriter(outputs...),
						TestHomePath:   home,
						TestPluginPath: testPluginPath,
						Env:            e,
						Plugin:         d,
					}
					td, err := testdriver.NewTestDriver(config)
					if err != nil {
						t.Fatalf("failed to create new driver: %v", err)
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
			}
			if err := gotooltest.Setup(&params); err != nil {
				t.Fatal(err)
			}
			testscript.Run(t, params)
		})
	}

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

func TestInstallScripts(t *testing.T) {
	t.Parallel()
	if os.Getenv(EnvInstallScripts) != "true" {
		t.Skipf("Skipping install scripts; %v != true", EnvInstallScripts)
	}

	govimPath := strings.TrimSpace(runCmd(t, "go", "list", "-m", "-f={{.Dir}}"))

	// For the tests where we set GOVIM_USE_GOPLS_FROM_PATH=true, install
	// gopls to a temp dir and add that dir to our PATH
	td, err := installGoplsToTempDir()
	if err != nil {
		t.Fatalf("failed to install gopls to temp directory: %v", err)
	}
	defer os.RemoveAll(td)

	t.Run("scripts", func(t *testing.T) {
		testscript.Run(t, testscript.Params{
			Dir: filepath.Join("testdata", "install"),
			Setup: func(e *testscript.Env) error {
				e.Vars = append(e.Vars,
					"PLUGIN_PATH="+govimPath,
					"GOPATH="+strings.TrimSpace(runCmd(t, "go", "env", "GOPATH")),
					"GOCACHE="+strings.TrimSpace(runCmd(t, "go", "env", "GOCACHE")),
					testsetup.EnvLoadTestAPI+"=true",
				)
				return nil
			},
		})
	})

	t.Run("scripts-with-gopls-from-path", func(t *testing.T) {
		testscript.Run(t, testscript.Params{
			Dir: filepath.Join("testdata", "install"),
			Setup: func(e *testscript.Env) error {
				var path string
				for i := len(e.Vars) - 1; i >= 0; i-- {
					v := e.Vars[i]
					if strings.HasPrefix(v, "PATH=") {
						path = strings.TrimPrefix(v, "PATH=")
						break
					}
				}
				if path == "" {
					path = td
				} else {
					path = td + string(os.PathListSeparator) + path
				}
				e.Vars = append(e.Vars,
					"PATH="+path,
					"PLUGIN_PATH="+govimPath,
					"GOPATH="+strings.TrimSpace(runCmd(t, "go", "env", "GOPATH")),
					"GOCACHE="+strings.TrimSpace(runCmd(t, "go", "env", "GOCACHE")),
					string(config.EnvVarUseGoplsFromPath)+"=true",
					testsetup.EnvLoadTestAPI+"=true",
				)
				return nil
			},
		})
	})
}

func runCmd(t *testing.T, c string, args ...string) string {
	t.Helper()
	cmd := exec.Command(c, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run %v: %v\n%s", strings.Join(cmd.Args, " "), err, out)
	}
	return string(out)
}

func execvim() int {
	args := append([]string{"--not-a-term"}, os.Args[1:]...)
	cmd := exec.Command("vim", args[1:]...)
	thepty, err := pty.Start(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start %v: %v", strings.Join(cmd.Args, " "), err)
		return 1
	}
	go io.Copy(ioutil.Discard, thepty)
	if err := cmd.Wait(); err != nil {
		return 1
	}
	return 0
}

func installGoplsToTempDir() (string, error) {
	td, err := ioutil.TempDir("", "gobin-gopls-installdir")
	if err != nil {
		return "", fmt.Errorf("failed to create temp install directory for gopls: %v", err)
	}
	cmd := exec.Command("go", "install", raceOrNot(), "golang.org/x/tools/gopls")
	cmd.Env = append(os.Environ(), "GOBIN="+td)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to install temp version of golang.org/x/tools/gopls: %v\n%s", err, out)
	}
	return td, nil
}

func readConfig(path string) (*config.Config, error) {
	// check whether we have a default config to apply
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return nil, nil
	}
	configByts, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %v: %v", path, err)
	}
	var res *config.Config
	if err := json.Unmarshal(configByts, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON config in %v: %v\n%s", path, err, configByts)
	}
	// Now verify that we haven't supplied superfluous JSON
	var i interface{}
	if err := json.Unmarshal(configByts, &i); err != nil {
		return nil, fmt.Errorf("failed to re-parse JSON config in %v: %v\n%s", path, err, configByts)
	}
	var resCheck []byte
	first, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to remarshal res: %v", err)
	}
	var j interface{}
	if err := json.Unmarshal(first, &j); err != nil {
		return nil, fmt.Errorf("failed to re-re-parse JSON config in %v: %v\n%s", path, err, configByts)
	}
	resCheck, err = json.MarshalIndent(j, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to remarshal res: %v", err)
	}
	iCheck, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to remarshal re-parsed JSON: %v", err)
	}
	if !bytes.Equal(resCheck, iCheck) {
		return nil, fmt.Errorf("%v contains superfluous JSON:\n\n%s\n\n vs parsed res:\n\n%s\n", path, iCheck, resCheck)
	}
	return res, nil
}
