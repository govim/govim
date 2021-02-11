// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package packagesinternal exposes internal-only fields from go/packages.
package packagesinternal

import (
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/gocommand"
)

var GetForTest = func(p interface{}) string { return "" }
var GetDepsErrors = func(p interface{}) []*PackageError { return nil }

type PackageError struct {
	ImportStack []string // shortest path from package named on command line to this one
	Pos         string   // position of error (if present, file:line:col)
	Err         string   // the error itself
}

var GetGoCmdRunner = func(config interface{}) *gocommand.Runner { return nil }

var SetGoCmdRunner = func(config interface{}, runner *gocommand.Runner) {}

var TypecheckCgo int

var SetModFlag = func(config interface{}, value string) {}
var SetModFile = func(config interface{}, value string) {}
