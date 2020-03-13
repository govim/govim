// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package export holds some exporter implementations.
// Larger more complex exporters are in sub packages of their own.
package export

import "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/telemetry/event"

var (
	SetExporter = event.SetExporter
)
