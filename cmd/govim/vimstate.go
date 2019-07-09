package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/config"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/types"
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

	// inBatch tracks whether we are gathering a batch of calls to Vim. Within a batch
	// only calls to the call channel function can be made.
	inBatch bool

	// currBatch represents the batch we are collecting whilst inBatch
	currBatch []interface{}
}

func (v *vimstate) setConfig(args ...json.RawMessage) (interface{}, error) {
	var c struct {
		FormatOnSave                   config.FormatOnSave
		QuickfixAutoDiagnosticsDisable int
	}
	v.Parse(args[0], &c)
	v.config = config.Config{
		FormatOnSave:                   c.FormatOnSave,
		QuickfixAutoDiagnosticsDisable: c.QuickfixAutoDiagnosticsDisable != 0,
	}
	return nil, nil
}

func (v *vimstate) hello(args ...json.RawMessage) (interface{}, error) {
	return "Hello from function", nil
}

func (v *vimstate) helloComm(flags govim.CommandFlags, args ...string) error {
	v.ChannelEx(`echom "Hello from command"`)
	return nil
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

func (v *vimstate) dumpPopups(args ...json.RawMessage) (interface{}, error) {
	var bufInfo []struct {
		BufNr  int   `json:"bufnr"`
		Popups []int `json:"popups"`
	}
	bi := v.ChannelExpr("getbufinfo()")
	v.Parse(bi, &bufInfo)
	sort.Slice(bufInfo, func(i, j int) bool {
		return bufInfo[i].BufNr < bufInfo[j].BufNr
	})
	var sb strings.Builder
	for _, b := range bufInfo {
		if len(b.Popups) == 0 {
			continue
		}
		sb.WriteString(v.ParseString(v.ChannelExprf(`join(getbufline(%v, 0, '$'), "\n")."\n"`, b.BufNr)))
	}
	return sb.String(), nil
}

func (v *vimstate) BatchStart() {
	if v.inBatch {
		panic(fmt.Errorf("called BatchStart whilst in a batch"))
	}
	v.inBatch = true
}

func (v *vimstate) ChannelExpr(expr string) json.RawMessage {
	if v.inBatch {
		panic(fmt.Errorf("cannot call ChannelExpr in batch"))
	}
	return v.Driver.ChannelExpr(expr)
}
func (v *vimstate) ChannelCall(name string, args ...interface{}) json.RawMessage {
	if v.inBatch {
		v.currBatch = append(v.currBatch, append([]interface{}{name}, args...))
		return nil
	} else {
		return v.Driver.ChannelCall(name, args...)
	}
}
func (v *vimstate) ChannelEx(expr string) {
	if v.inBatch {
		panic(fmt.Errorf("cannot call ChannelEx in batch"))
	}
	v.Driver.ChannelEx(expr)
}
func (v *vimstate) ChannelNormal(expr string) {
	if v.inBatch {
		panic(fmt.Errorf("cannot call ChannelNormal in batch"))
	}
	v.Driver.ChannelNormal(expr)
}
func (v *vimstate) ChannelRedraw(force bool) {
	if v.inBatch {
		panic(fmt.Errorf("cannot call ChannelRedraw in batch"))
	}
	v.Driver.ChannelRedraw(force)
}
func (v *vimstate) ChannelExprf(format string, args ...interface{}) json.RawMessage {
	if v.inBatch {
		panic(fmt.Errorf("cannot call ChannelExprf in batch"))
	}
	return v.Driver.ChannelExprf(format, args...)
}
func (v *vimstate) ChannelExf(format string, args ...interface{}) {
	if v.inBatch {
		panic(fmt.Errorf("cannot call ChannelExf in batch"))
	}
	v.Driver.ChannelExf(format, args...)
}

func (v *vimstate) BatchEnd() (res []json.RawMessage) {
	if !v.inBatch {
		panic(fmt.Errorf("called BatchEnd but not in a batch"))
	}
	v.inBatch = false
	calls := v.currBatch
	v.currBatch = nil
	if len(calls) == 0 {
		return
	}
	vs := v.ChannelCall("s:batchCall", calls)
	v.Parse(vs, &res)
	return
}
