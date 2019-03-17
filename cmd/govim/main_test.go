// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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
