// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package regtest provides an environment for writing regression tests.
package regtest

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/jsonrpc2/servertest"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/cache"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/fake"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/lsprpc"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
)

// EnvMode is a bitmask that defines in which execution environments a test
// should run.
type EnvMode int

const (
	// Singleton mode uses a separate cache for each test
	Singleton EnvMode = 1 << iota
	// Shared mode uses a Shared cache
	Shared
	// Forwarded forwards connections
	Forwarded
	// AllModes runs tests in all modes
	AllModes = Singleton | Shared | Forwarded
)

// A Runner runs tests in gopls execution environments, as specified by its
// modes. For modes that share state (for example, a shared cache or common
// remote), any tests that execute on the same Runner will share the same
// state.
type Runner struct {
	ts      *servertest.Server
	modes   EnvMode
	timeout time.Duration
}

// NewTestRunner creates a Runner with its shared state initialized, ready to
// run tests.
func NewTestRunner(modes EnvMode, testTimeout time.Duration) *Runner {
	ss := lsprpc.NewStreamServer(cache.New(nil), false)
	ts := servertest.NewServer(context.Background(), ss)
	return &Runner{
		ts:      ts,
		modes:   modes,
		timeout: testTimeout,
	}
}

// Close cleans up resource that have been allocated to this workspace.
func (r *Runner) Close() error {
	return r.ts.Close()
}

// Run executes the test function in in all configured gopls execution modes.
// For each a test run, a new workspace is created containing the un-txtared
// files specified by filedata.
func (r *Runner) Run(t *testing.T, filedata string, test func(context.Context, *testing.T, *Env)) {
	t.Helper()

	tests := []struct {
		name       string
		mode       EnvMode
		makeServer func(context.Context, *testing.T) (*servertest.Server, func())
	}{
		{"singleton", Singleton, r.singletonEnv},
		{"shared", Shared, r.sharedEnv},
		{"forwarded", Forwarded, r.forwardedEnv},
	}

	for _, tc := range tests {
		tc := tc
		if r.modes&tc.mode == 0 {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
			defer cancel()
			ws, err := fake.NewWorkspace("lsprpc", []byte(filedata))
			if err != nil {
				t.Fatal(err)
			}
			defer ws.Close()
			ts, cleanup := tc.makeServer(ctx, t)
			defer cleanup()
			env := NewEnv(ctx, t, ws, ts)
			test(ctx, t, env)
		})
	}
}

func (r *Runner) singletonEnv(ctx context.Context, t *testing.T) (*servertest.Server, func()) {
	ss := lsprpc.NewStreamServer(cache.New(nil), false)
	ts := servertest.NewServer(ctx, ss)
	cleanup := func() {
		ts.Close()
	}
	return ts, cleanup
}

func (r *Runner) sharedEnv(ctx context.Context, t *testing.T) (*servertest.Server, func()) {
	return r.ts, func() {}
}

func (r *Runner) forwardedEnv(ctx context.Context, t *testing.T) (*servertest.Server, func()) {
	forwarder := lsprpc.NewForwarder(r.ts.Addr, false)
	ts2 := servertest.NewServer(ctx, forwarder)
	cleanup := func() {
		ts2.Close()
	}
	return ts2, cleanup
}

// Env holds an initialized fake Editor, Workspace, and Server, which may be
// used for writing tests. It also provides adapter methods that call t.Fatal
// on any error, so that tests for the happy path may be written without
// checking errors.
type Env struct {
	t   *testing.T
	ctx context.Context

	// Most tests should not need to access the workspace or editor, or server,
	// but they are available if needed.
	W      *fake.Workspace
	E      *fake.Editor
	Server *servertest.Server

	// mu guards the fields below, for the purpose of checking conditions on
	// every change to diagnostics.
	mu sync.Mutex
	// For simplicity, each waiter gets a unique ID.
	nextWaiterID    int
	lastDiagnostics map[string]*protocol.PublishDiagnosticsParams
	waiters         map[int]*diagnosticCondition
}

// A diagnosticCondition is satisfied when all expectations are simultaneously
// met. At that point, the 'met' channel is closed.
type diagnosticCondition struct {
	expectations []DiagnosticExpectation
	met          chan struct{}
}

// NewEnv creates a new test environment using the given workspace and gopls
// server.
func NewEnv(ctx context.Context, t *testing.T, ws *fake.Workspace, ts *servertest.Server) *Env {
	t.Helper()
	conn := ts.Connect(ctx)
	editor, err := fake.NewConnectedEditor(ctx, ws, conn)
	if err != nil {
		t.Fatal(err)
	}
	env := &Env{
		t:               t,
		ctx:             ctx,
		W:               ws,
		E:               editor,
		Server:          ts,
		lastDiagnostics: make(map[string]*protocol.PublishDiagnosticsParams),
		waiters:         make(map[int]*diagnosticCondition),
	}
	env.E.Client().OnDiagnostics(env.onDiagnostics)
	return env
}

// RemoveFileFromWorkspace deletes a file on disk but does nothing in the
// editor. It calls t.Fatal on any error.
func (e *Env) RemoveFileFromWorkspace(name string) {
	e.t.Helper()
	if err := e.W.RemoveFile(e.ctx, name); err != nil {
		e.t.Fatal(err)
	}
}

// OpenFile opens a file in the editor, calling t.Fatal on any error.
func (e *Env) OpenFile(name string) {
	e.t.Helper()
	if err := e.E.OpenFile(e.ctx, name); err != nil {
		e.t.Fatal(err)
	}
}

// CreateBuffer creates a buffer in the editor, calling t.Fatal on any error.
func (e *Env) CreateBuffer(name string, content string) {
	e.t.Helper()
	if err := e.E.CreateBuffer(e.ctx, name, content); err != nil {
		e.t.Fatal(err)
	}
}

// EditBuffer applies edits to an editor buffer, calling t.Fatal on any error.
func (e *Env) EditBuffer(name string, edits ...fake.Edit) {
	e.t.Helper()
	if err := e.E.EditBuffer(e.ctx, name, edits); err != nil {
		e.t.Fatal(err)
	}
}

// GoToDefinition goes to definition in the editor, calling t.Fatal on any
// error.
func (e *Env) GoToDefinition(name string, pos fake.Pos) (string, fake.Pos) {
	e.t.Helper()
	n, p, err := e.E.GoToDefinition(e.ctx, name, pos)
	if err != nil {
		e.t.Fatal(err)
	}
	return n, p
}

func (e *Env) onDiagnostics(_ context.Context, d *protocol.PublishDiagnosticsParams) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	pth := e.W.URIToPath(d.URI)
	e.lastDiagnostics[pth] = d

	for id, condition := range e.waiters {
		if meetsCondition(e.lastDiagnostics, condition.expectations) {
			delete(e.waiters, id)
			close(condition.met)
		}
	}

	return nil
}

func meetsCondition(m map[string]*protocol.PublishDiagnosticsParams, expectations []DiagnosticExpectation) bool {
	for _, e := range expectations {
		if !e.IsMet(m) {
			return false
		}
	}
	return true
}

// A DiagnosticExpectation is a condition that must be met by the current set
// of diagnostics.
type DiagnosticExpectation struct {
	IsMet       func(map[string]*protocol.PublishDiagnosticsParams) bool
	Description string
}

// EmptyDiagnostics asserts that diagnostics are empty for the
// workspace-relative path name.
func EmptyDiagnostics(name string) DiagnosticExpectation {
	isMet := func(diags map[string]*protocol.PublishDiagnosticsParams) bool {
		ds, ok := diags[name]
		return ok && len(ds.Diagnostics) == 0
	}
	return DiagnosticExpectation{
		IsMet:       isMet,
		Description: fmt.Sprintf("empty diagnostics for %q", name),
	}
}

// DiagnosticAt asserts that there is a diagnostic entry at the position
// specified by line and col, for the workspace-relative path name.
func DiagnosticAt(name string, line, col int) DiagnosticExpectation {
	isMet := func(diags map[string]*protocol.PublishDiagnosticsParams) bool {
		ds, ok := diags[name]
		if !ok || len(ds.Diagnostics) == 0 {
			return false
		}
		for _, d := range ds.Diagnostics {
			if d.Range.Start.Line == float64(line) && d.Range.Start.Character == float64(col) {
				return true
			}
		}
		return false
	}
	return DiagnosticExpectation{
		IsMet:       isMet,
		Description: fmt.Sprintf("diagnostic in %q at (line:%d, column:%d)", name, line, col),
	}
}

// Await waits for all diagnostic expectations to simultaneously be met.
func (e *Env) Await(expectations ...DiagnosticExpectation) {
	// NOTE: in the future this mechanism extend beyond just diagnostics, for
	// example by modifying IsMet to be a func(*Env) boo.  However, that would
	// require careful checking of conditions around every state change, so for
	// now we just limit the scope to diagnostic conditions.

	e.t.Helper()
	e.mu.Lock()
	// Before adding the waiter, we check if the condition is currently met to
	// avoid a race where the condition was realized before Await was called.
	if meetsCondition(e.lastDiagnostics, expectations) {
		e.mu.Unlock()
		return
	}
	met := make(chan struct{})
	e.waiters[e.nextWaiterID] = &diagnosticCondition{
		expectations: expectations,
		met:          met,
	}
	e.nextWaiterID++
	e.mu.Unlock()

	select {
	case <-e.ctx.Done():
		// Debugging an unmet expectation can be tricky, so we put some effort into
		// nicely formatting the failure.
		var descs []string
		for _, e := range expectations {
			descs = append(descs, e.Description)
		}
		e.mu.Lock()
		diagString := formatDiagnostics(e.lastDiagnostics)
		e.mu.Unlock()
		e.t.Fatalf("waiting on (%s):\nerr:%v\ndiagnostics:\n%s", strings.Join(descs, ", "), e.ctx.Err(), diagString)
	case <-met:
	}
}

func formatDiagnostics(diags map[string]*protocol.PublishDiagnosticsParams) string {
	var b strings.Builder
	for name, params := range diags {
		b.WriteString(name + ":\n")
		for _, d := range params.Diagnostics {
			b.WriteString(fmt.Sprintf("\t(%d, %d): %s\n", int(d.Range.Start.Line), int(d.Range.Start.Character), d.Message))
		}
	}
	return b.String()
}
