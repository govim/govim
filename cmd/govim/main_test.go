// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
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

	"github.com/kr/pty"
	"github.com/myitcv/govim/testdriver"
	"github.com/rogpeppe/go-internal/testscript"
)

var (
	fLogGovim = flag.Bool("govimLog", false, "whether to log govim activity")
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"vim":     testdriver.Vim,
		"execvim": execvim,
	}))
}

func TestScripts(t *testing.T) {
	var waitLock sync.Mutex
	var waitList []func() error

	td, err := ioutil.TempDir("", "gobin-gopls-installdir*")
	if err != nil {
		t.Fatalf("failed to create temp install directory for gopls: %v", err)
	}
	defer os.RemoveAll(td)

	cmd := exec.Command("go", "install", "golang.org/x/tools/cmd/gopls")
	cmd.Env = append(os.Environ(), "GOBIN="+td)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to install temp version of golang.org/x/tools/cmd/gopls: %v\n%s", err, out)
	}

	goplspath := filepath.Join(td, "gopls")
	plugpath := strings.TrimSpace(runCmd(t, "go", "list", "-m", "-f={{.Dir}}"))

	t.Run("scripts", func(t *testing.T) {
		testscript.Run(t, testscript.Params{
			Dir: "testdata",
			Setup: func(e *testscript.Env) error {
				e.Vars = append(e.Vars,
					"PLUGIN_PATH="+plugpath,
					"CURRENT_GOPATH="+os.Getenv("GOPATH"),
				)
				d := newplugin(string(goplspath))
				td, err := testdriver.NewTestDriver(filepath.Base(e.WorkDir), e, d)
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
				td.Run()
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

func TestInstallScripts(t *testing.T) {
	if testing.Short() {
		t.Skip("Install scripts are long-running")
	}

	plugpath := strings.TrimSpace(runCmd(t, "go", "list", "-m", "-f={{.Dir}}"))

	t.Run("scripts", func(t *testing.T) {
		testscript.Run(t, testscript.Params{
			Dir: "testdatainstall",
			Setup: func(e *testscript.Env) error {
				e.Vars = append(e.Vars,
					"PLUGIN_PATH="+plugpath,
					"CURRENT_GOPATH="+os.Getenv("GOPATH"),
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
