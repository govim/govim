// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmdtest

import (
	"fmt"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/cmd"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/tool"
	"testing"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

func (r *runner) References(t *testing.T, spn span.Span, itemList []span.Span) {
	var expect string
	for _, i := range itemList {
		expect += fmt.Sprintln(i)
	}

	uri := spn.URI()
	filename := uri.Filename()
	target := filename + fmt.Sprintf(":%v:%v", spn.Start().Line(), spn.Start().Column())

	app := cmd.New("gopls-test", r.data.Config.Dir, r.data.Config.Env, r.options)
	got := CaptureStdOut(t, func() {
		err := tool.Run(r.ctx, app, append([]string{"-remote=internal", "references"}, target))
		if err != nil {
			fmt.Println(spn.Start().Line())
			fmt.Println(err)
		}
	})

	if expect != got {
		t.Errorf("references failed for %s expected:\n%s\ngot:\n%s", target, expect, got)
	}
}
