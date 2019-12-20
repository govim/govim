// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmdtest

import (
	"testing"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

func (r *runner) Import(t *testing.T, spn span.Span) {
	uri := spn.URI()
	filename := uri.Filename()
	got, _ := r.NormalizeGoplsCmd(t, "imports", filename)
	want := string(r.data.Golden("goimports", filename, func() ([]byte, error) {
		return []byte(got), nil
	}))
	if want != got {
		t.Errorf("imports failed for %s, expected:\n%q\ngot:\n%q", filename, want, got)
	}
}
