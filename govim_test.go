// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package govim_test

import (
	"encoding/json"
	"fmt"
	"os"
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
				d, err := testdriver.NewDriver(e, errCh, func(g *govim.Govim) error {
					if err := g.DefineFunction("Hello", []string{}, hello); err != nil {
						return fmt.Errorf("failed to DefineFunction %q: %v", "Hello", err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("failed to create new driver: %v", err)
				}
				d.Run()
				e.Defer(func() {
					d.Close()
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

func hello(args ...json.RawMessage) (interface{}, error) {
	return "World", nil
}
