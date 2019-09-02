package main

import (
	"encoding/json"
	"fmt"

	"github.com/myitcv/govim/cmd/govim/config"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/types"
	"github.com/myitcv/govim/internal/plugin"
)

type vimstate struct {
	plugin.Driver
	*govimplugin

	// buffers represents the current state of all buffers in Vim. It is only safe to
	// write and read to/from this map in the callback for a defined function, command
	// or autocommand.
	buffers map[int]*types.Buffer

	// watchedFiles is a map of files that we are handling via file watching
	// events, rather than via open Buffers in Vim
	watchedFiles map[string]*types.WatchedFile

	// jumpStack is akin to the Vim concept of a tagstack
	jumpStack    []protocol.Location
	jumpStackPos int

	// omnifunc calls happen in pairs (see :help complete-functions). The return value
	// from the first tells Vim where the completion starts, the return from the second
	// returns the matching words. This is by definition stateful. Hence we persist that
	// state here
	lastCompleteResults *protocol.CompletionList

	config config.Config

	// userBusy indicates the user is moving the cusor doing something
	userBusy bool

	// quickfixIsDiagnostics is a flag that indicates the quickfix window is being
	// used for diagnostics, and not, for example, locations of references. If
	// the user calls GOVIMReferences, quickfixIsDiagnostics is set to false; whilst
	// false the quickfix window will not update with diagnostics, until the user
	// calls GOVIMQuickfixDiagnostics, which sets the flag to true.
	quickfixIsDiagnostics bool

	// diagnosticsChanged indicates that the quickfix window needs to be updated with
	// the latest diagnostics
	diagnosticsChanged bool

	// popupWinId is the id of the window currently being used for a hover-based popup
	popupWinId int

	// currBatch represents the batch we are collecting
	currBatch *batch

	// lastCompleteResults is the last set of error diagnostics we set as quickfix
	// entries. We use this in order to retain the index in the quickfix list when the
	// quickfix entries change. If the previously selected entry remains in the new
	// quickfix list we re-select it. Otherwise we select the first entry.
	lastQuickFixDiagnostics []quickfixEntry
}

func (v *vimstate) setConfig(args ...json.RawMessage) (interface{}, error) {
	preConfig := v.config
	var c struct {
		FormatOnSave                                 config.FormatOnSave
		QuickfixAutoDiagnosticsDisable               int
		QuickfixSignsDisable                         int
		ExperimentalMouseTriggeredHoverPopupOptions  map[string]json.RawMessage
		ExperimentalCursorTriggeredHoverPopupOptions map[string]json.RawMessage
	}
	v.Parse(args[0], &c)
	v.config = config.Config{
		FormatOnSave:                   c.FormatOnSave,
		QuickfixSignsDisable:           c.QuickfixSignsDisable != 0,
		QuickfixAutoDiagnosticsDisable: c.QuickfixAutoDiagnosticsDisable != 0,
	}
	if len(c.ExperimentalMouseTriggeredHoverPopupOptions) > 0 {
		v.config.ExperimentalMouseTriggeredHoverPopupOptions = make(map[string]interface{})
		for ck, cv := range c.ExperimentalMouseTriggeredHoverPopupOptions {
			v.config.ExperimentalMouseTriggeredHoverPopupOptions[ck] = cv
		}
	}
	if len(c.ExperimentalCursorTriggeredHoverPopupOptions) > 0 {
		v.config.ExperimentalCursorTriggeredHoverPopupOptions = make(map[string]interface{})
		for ck, cv := range c.ExperimentalCursorTriggeredHoverPopupOptions {
			v.config.ExperimentalCursorTriggeredHoverPopupOptions[ck] = cv
		}
	}
	if v.config.QuickfixAutoDiagnosticsDisable != preConfig.QuickfixAutoDiagnosticsDisable ||
		v.config.QuickfixSignsDisable != preConfig.QuickfixSignsDisable {
		if v.config.QuickfixAutoDiagnosticsDisable {
			v.lastQuickFixDiagnostics = []quickfixEntry{}
			if v.quickfixIsDiagnostics {
				v.ChannelCall("setqflist", v.lastQuickFixDiagnostics, "r")
			}
		} else {
			v.diagnosticsLock.Lock()
			v.diagnosticsChanged = true
			v.diagnosticsLock.Unlock()
			return nil, v.updateQuickfix()
		}
	}
	return nil, nil
}

func (v *vimstate) setUserBusy(args ...json.RawMessage) (interface{}, error) {
	var isBusy int
	v.Parse(args[0], &isBusy)
	v.userBusy = isBusy != 0
	if v.userBusy || v.config.QuickfixAutoDiagnosticsDisable || !v.quickfixIsDiagnostics {
		return nil, nil
	}
	return nil, v.updateQuickfix()
}

type batch struct {
	calls   []interface{}
	results []json.RawMessage
}

func (b *batch) result() batchResult {
	i := len(b.calls) - 1
	return func() json.RawMessage {
		if b.results == nil {
			panic(fmt.Errorf("tried to get result from incomplete Batch"))
		}
		return b.results[i]
	}
}

func (v *vimstate) BatchStart() {
	if v.currBatch != nil {
		panic(fmt.Errorf("called BatchStart whilst in a batch"))
	}
	v.currBatch = &batch{}
}

type batchResult func() json.RawMessage

type AssertExpr string

const (
	AssertNothing AssertExpr = "s:mustNothing"
	AssertIsZero  AssertExpr = "s:mustBeZero"
)

func (v *vimstate) BatchChannelExprf(format string, args ...interface{}) batchResult {
	return v.BatchAssertChannelExprf(AssertNothing, format, args...)
}

func (v *vimstate) BatchAssertChannelExprf(m AssertExpr, format string, args ...interface{}) batchResult {
	if v.currBatch == nil {
		panic(fmt.Errorf("cannot call BatchChannelExprf: not in batch"))
	}
	v.currBatch.calls = append(v.currBatch.calls, []interface{}{
		"expr",
		m,
		fmt.Sprintf(format, args...),
	})
	return v.currBatch.result()
}
func (v *vimstate) BatchChannelCall(name string, args ...interface{}) batchResult {
	return v.BatchAssertChannelCall(AssertNothing, name, args...)
}

func (v *vimstate) BatchAssertChannelCall(a AssertExpr, name string, args ...interface{}) batchResult {
	if v.currBatch == nil {
		panic(fmt.Errorf("cannot call BatchChannelCall: not in batch"))
	}
	callargs := []interface{}{
		"call",
		a,
		name,
	}
	callargs = append(callargs, args...)
	v.currBatch.calls = append(v.currBatch.calls, callargs)
	return v.currBatch.result()
}

func (v *vimstate) BatchCancelIfNotEnded() {
	v.currBatch = nil
}

func (v *vimstate) BatchEnd() (res []json.RawMessage) {
	if v.currBatch == nil {
		panic(fmt.Errorf("called BatchEnd but not in a batch"))
	}
	b := v.currBatch
	v.currBatch = nil
	if len(b.calls) == 0 {
		return
	}
	vs := v.ChannelCall("s:batchCall", b.calls)
	v.Parse(vs, &res)
	b.results = res
	return
}
