// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmdtest

import (
	"fmt"
	"testing"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/tests"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/tests/compare"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

func (r *runner) SuggestedFix(t *testing.T, spn span.Span, suggestedFixes []tests.SuggestedFix, expectedActions int) {
	uri := spn.URI()
	filename := uri.Filename()
	args := []string{"fix", "-a", fmt.Sprintf("%s", spn)}
	var actionKinds []string
	for _, sf := range suggestedFixes {
		if sf.ActionKind == "refactor.rewrite" {
			t.Skip("refactor.rewrite is not yet supported on the command line")
		}
		actionKinds = append(actionKinds, sf.ActionKind)
	}
	args = append(args, actionKinds...)
	got, stderr := r.NormalizeGoplsCmd(t, args...)
	if stderr == "ExecuteCommand is not yet supported on the command line" {
		return // don't skip to keep the summary counts correct
	}
	want := string(r.data.Golden("suggestedfix_"+tests.SpanName(spn), filename, func() ([]byte, error) {
		return []byte(got), nil
	}))
	if want != got {
		t.Errorf("suggested fixes failed for %s:\n%s", filename, compare.Text(want, got))
	}
}
