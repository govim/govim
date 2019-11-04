// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmdtest

import (
	"testing"

	"fmt"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/cmd"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/tool"
)

func (r *runner) Symbols(t *testing.T, uri span.URI, expectedSymbols []protocol.DocumentSymbol) {
	filename := uri.Filename()
	app := cmd.New("gopls-test", r.data.Config.Dir, r.data.Config.Env, r.options)
	got := CaptureStdOut(t, func() {
		err := tool.Run(r.ctx, app, append([]string{"-remote=internal", "symbols"}, filename))
		if err != nil {
			fmt.Println(err)
		}
	})
	expect := string(r.data.Golden("symbols", filename, func() ([]byte, error) {
		return []byte(got), nil
	}))
	if expect != got {
		t.Errorf("symbols failed for %s expected:\n%s\ngot:\n%s", filename, expect, got)
	}
}
