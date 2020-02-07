// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmdtest

import (
	"fmt"
	"testing"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

func (r *runner) SignatureHelp(t *testing.T, spn span.Span, want *protocol.SignatureHelp) {
	uri := spn.URI()
	filename := uri.Filename()
	target := filename + fmt.Sprintf(":%v:%v", spn.Start().Line(), spn.Start().Column())
	got, _ := r.NormalizeGoplsCmd(t, "signature", target)
	if want == nil {
		if got != "" {
			t.Fatalf("want nil, but got %s", got)
		}
		return
	}
	goldenTag := want.Signatures[0].Label + "-signature"
	expect := string(r.data.Golden(goldenTag, filename, func() ([]byte, error) {
		return []byte(got), nil
	}))
	if expect != got {
		t.Errorf("signature failed for %s expected:\n%q\ngot:\n%q'", filename, expect, got)
	}
}
