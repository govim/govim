package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/types"
	"github.com/govim/govim/cmd/govim/internal/vimconfig"
	"github.com/govim/govim/internal/plugin"
)

type vimstate struct {
	plugin.Driver
	*govimplugin

	// buffers represents the current state of all buffers in Vim. It is only safe to
	// write and read to/from this map in the callback for a defined function, command
	// or autocommand.
	buffers map[int]*types.Buffer

	// jumpStack is akin to the Vim concept of a tagstack
	jumpStack    []protocol.Location
	jumpStackPos int

	// omnifunc calls happen in pairs (see :help complete-functions). The return value
	// from the first tells Vim where the completion starts, the return from the second
	// returns the matching words. This is by definition stateful. Hence we persist that
	// state here
	lastCompleteResults *protocol.CompletionList

	defaultConfig config.Config
	config        config.Config
	configLock    sync.Mutex

	// userBusy indicates the user is moving the cusor doing something
	userBusy bool

	// quickfixIsDiagnostics is a flag that indicates the quickfix window is being
	// used for diagnostics, and not, for example, locations of references. If
	// the user calls GOVIMReferences, quickfixIsDiagnostics is set to false; whilst
	// false the quickfix window will not update with diagnostics, until the user
	// calls GOVIMQuickfixDiagnostics, which sets the flag to true.
	quickfixIsDiagnostics bool

	// popupWinId is the id of the window currently being used for a hover-based popup
	popupWinId int

	// currBatch represents the batch we are collecting
	currBatch *batch

	// lastCompleteResults is the last set of error diagnostics we set as quickfix
	// entries. We use this in order to retain the index in the quickfix list when the
	// quickfix entries change. If the previously selected entry remains in the new
	// quickfix list we re-select it. Otherwise we select the first entry.
	lastQuickFixDiagnostics []quickfixEntry

	// suggestedFixesPopups is a set of suggested fixes keyed by popup ID. It represents
	// currently defined popups (both hidden and visible) and have a lifespan of single
	// codeAction call.
	suggestedFixesPopups map[int][]protocol.WorkspaceEdit

	// working directory (when govim was started)
	// TODO: handle changes to current working directory during runtime
	workingDirectory string
}

func (v *vimstate) setConfig(args ...json.RawMessage) (interface{}, error) {
	preConfig := v.config
	var vc vimconfig.VimConfig
	v.Parse(args[0], &vc)
	v.configLock.Lock()
	v.config = vc.ToConfig(v.defaultConfig)
	v.configLock.Unlock()

	// Remember: the boolean value fields are effectively tri-state. Because they
	// are actually *bool.

	if !vimconfig.EqualBool(v.config.QuickfixAutoDiagnostics, preConfig.QuickfixAutoDiagnostics) {
		if v.config.QuickfixAutoDiagnostics == nil || !*v.config.QuickfixAutoDiagnostics {
			// QuickfixAutoDiagnostics is now not on
			v.lastQuickFixDiagnostics = []quickfixEntry{}
			if v.quickfixIsDiagnostics {
				v.ChannelCall("setqflist", v.lastQuickFixDiagnostics, "r")
			}
		} else {
			// QuickfixAutoDiagnostics is now on
			if err := v.updateQuickfixWithDiagnostics(true, false); err != nil {
				return nil, fmt.Errorf("failed to update diagnostics: %v", err)
			}
		}
	}

	if !vimconfig.EqualBool(v.config.QuickfixSigns, preConfig.QuickfixSigns) {
		if v.config.QuickfixSigns == nil || !*v.config.QuickfixSigns {
			// QuickfixSigns is now not on - clear all signs
			v.ChannelCall("sign_unplace", signGroup)
		} else {
			// QuickfixSigns is now on
			if err := v.updateSigns(true); err != nil {
				return nil, fmt.Errorf("failed to update placed signs: %v", err)
			}
		}
	}

	if !vimconfig.EqualBool(v.config.HighlightDiagnostics, preConfig.HighlightDiagnostics) {
		if v.config.HighlightDiagnostics == nil || !*v.config.HighlightDiagnostics {
			// HighlightDiagnostics is now not on - remove existing text properties
			v.removeTextProps(types.DiagnosticTextPropID)
		} else {
			if err := v.redefineHighlights(true); err != nil {
				return nil, fmt.Errorf("failed to update diagnostic highlights: %v", err)
			}
		}
	}

	if !vimconfig.EqualBool(v.config.HighlightReferences, preConfig.HighlightReferences) {
		if v.config.HighlightReferences == nil || !*v.config.HighlightReferences {
			// HighlightReferences is now not on - remove existing text properties
			v.removeTextProps(types.ReferencesTextPropID)
		} else {
			if err := v.updateReferenceHighlight(true, nil); err != nil {
				return nil, fmt.Errorf("failed to update reference highlight: %v", err)
			}
		}
	}

	// v.server will be nil when we are Init()-ing govim. The init process
	// triggers a "manual" call of govim#config#Set() and hence this function
	// gets called before we have even started gopls.
	var err error
	if v.server != nil {
		err = v.server.DidChangeConfiguration(context.Background(), &protocol.DidChangeConfigurationParams{})
	}

	return nil, err
}

func (v *vimstate) popupSelection(args ...json.RawMessage) (interface{}, error) {
	var popupID int
	var selection int
	v.Parse(args[0], &popupID)
	v.Parse(args[1], &selection)

	var edits []protocol.WorkspaceEdit
	var ok bool
	if edits, ok = v.suggestedFixesPopups[popupID]; !ok {
		return nil, fmt.Errorf("couldn't find popup id: %d", popupID)
	}

	delete(v.suggestedFixesPopups, popupID)

	for k := range v.suggestedFixesPopups {
		v.ChannelCall("popup_close", k)
		delete(v.suggestedFixesPopups, popupID)
	}

	if selection < 1 { // 0 = popup_close() called, -1 = ESC closed popup
		return nil, nil
	}

	edit := edits[selection-1]

	return nil, v.applyMultiBufTextedits(nil, edit.DocumentChanges)
}

func (v *vimstate) setUserBusy(args ...json.RawMessage) (interface{}, error) {
	v.userBusy = v.ParseInt(args[0]) != 0
	var pos cursorPosition
	v.Parse(args[1], &pos)

	if v.userBusy {
		return nil, v.removeReferenceHighlight(pos)
	}

	if err := v.updateReferenceHighlight(false, &pos); err != nil {
		return nil, err
	}

	return nil, v.handleDiagnosticsChanged()
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

func (v *vimstate) BatchStartIfNeeded() bool {
	if v.currBatch != nil {
		return false
	}
	v.currBatch = &batch{}
	return true
}

type batchResult func() json.RawMessage

type AssertExpr struct {
	Fn   string
	Args []interface{}
}

func AssertNoError() AssertExpr {
	return AssertExpr{
		Fn: "s:mustNoError",
	}
}

func AssertIsZero() AssertExpr {
	return AssertExpr{
		Fn: "s:mustBeZero",
	}
}

func AssertIsErrorOrNil(patterns ...string) AssertExpr {
	args := make([]interface{}, 0, len(patterns))
	for _, v := range patterns {
		args = append(args, v)
	}
	return AssertExpr{
		Fn:   "s:mustBeErrorOrNil",
		Args: args,
	}
}

func (v *vimstate) BatchChannelExprf(format string, args ...interface{}) batchResult {
	return v.BatchAssertChannelExprf(AssertNoError(), format, args...)
}

func (v *vimstate) BatchAssertChannelExprf(a AssertExpr, format string, args ...interface{}) batchResult {
	if v.currBatch == nil {
		panic(fmt.Errorf("cannot call BatchChannelExprf: not in batch"))
	}
	v.currBatch.calls = append(v.currBatch.calls, []interface{}{
		"expr",
		[2]interface{}{a.Fn, a.Args},
		fmt.Sprintf(format, args...),
	})
	return v.currBatch.result()
}
func (v *vimstate) BatchChannelCall(name string, args ...interface{}) batchResult {
	return v.BatchAssertChannelCall(AssertNoError(), name, args...)
}

func (v *vimstate) BatchAssertChannelCall(a AssertExpr, name string, args ...interface{}) batchResult {
	if v.currBatch == nil {
		panic(fmt.Errorf("cannot call BatchChannelCall: not in batch"))
	}
	callargs := []interface{}{
		"call",
		[2]interface{}{a.Fn, a.Args},
		name,
	}
	callargs = append(callargs, args...)
	v.currBatch.calls = append(v.currBatch.calls, callargs)
	return v.currBatch.result()
}

func (v *vimstate) BatchCancelIfNotEnded() {
	v.currBatch = nil
}

func (v *vimstate) BatchEnd() ([]json.RawMessage, error) {
	return v.batchEndImpl(false)
}

func (v *vimstate) MustBatchEnd() (res []json.RawMessage) {
	res, _ = v.batchEndImpl(true)
	return
}

func (v *vimstate) batchEndImpl(must bool) (res []json.RawMessage, err error) {
	if v.currBatch == nil {
		panic(fmt.Errorf("called BatchEnd but not in a batch"))
	}
	b := v.currBatch
	v.currBatch = nil
	if len(b.calls) == 0 {
		return
	}
	var vs json.RawMessage
	if must {
		vs = v.ChannelCall("s:batchCall", b.calls)
	} else {
		vs, err = v.Driver.Govim.ChannelCall("s:batchCall", b.calls)
		if err != nil {
			return
		}
	}
	v.Parse(vs, &res)
	b.results = res
	return
}

func (v *vimstate) ChannelCall(name string, args ...interface{}) json.RawMessage {
	if v.currBatch != nil {
		panic(fmt.Errorf("called ChannelCall when in batch"))
	}
	return v.Driver.ChannelCall(name, args...)
}

func (v *vimstate) ChannelEx(expr string) {
	if v.currBatch != nil {
		panic(fmt.Errorf("called ChannelEx when in batch"))
	}
	v.Driver.ChannelEx(expr)
}

func (v *vimstate) ChannelExf(format string, args ...interface{}) {
	if v.currBatch != nil {
		panic(fmt.Errorf("called ChannelExf when in batch"))
	}
	v.Driver.ChannelExf(format, args...)
}

func (v *vimstate) ChannelExpr(expr string) json.RawMessage {
	if v.currBatch != nil {
		panic(fmt.Errorf("called ChannelExpr when in batch"))
	}
	return v.Driver.ChannelExpr(expr)
}

func (v *vimstate) ChannelExprf(format string, args ...interface{}) json.RawMessage {
	if v.currBatch != nil {
		panic(fmt.Errorf("called ChannelExprf when in batch"))
	}
	return v.Driver.ChannelExprf(format, args...)
}

func (v *vimstate) ChannelNormal(expr string) {
	if v.currBatch != nil {
		panic(fmt.Errorf("called ChannelNormal when in batch"))
	}
	v.Driver.ChannelNormal(expr)
}

func (v *vimstate) ChannelRedraw(force bool) {
	if v.currBatch != nil {
		panic(fmt.Errorf("called ChannelRedraw when in batch"))
	}
	v.Driver.ChannelRedraw(force)
}
