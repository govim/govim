package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
)

// This file contains config that would otherwise be in the
// github.com/govim/govim/cmd/govim/config, but for the fact that these are
// only definitions we need for the purposes of testing

const (
	CommandHello config.Command = "Hello"
)

const (
	FunctionHello               config.Function = "Hello"
	FunctionDumpPopups          config.Function = config.InternalFunctionPrefix + "DumpPopups"
	FunctionSimpleBatch         config.Function = "SimpleBatch"
	FunctionCancelBatch         config.Function = "CancelBatch"
	FunctionBadBatch            config.Function = "BadBatch"
	FunctionAssertFailedBatch   config.Function = "AssertFailedBatch"
	FunctionNonBatchCallInBatch config.Function = "NonBatchCallInBatch"
	FunctionIgnoreErrorInBatch  config.Function = "IgnoreErrorInBatch"
	FunctionShowMessagePopup    config.Function = config.InternalFunctionPrefix + "ShowMessagePopup"
)

func (g *govimplugin) InitTestAPI() {
	if !exposeTestAPI {
		return
	}

	g.DefineFunction(string(FunctionHello), []string{}, g.vimstate.hello)
	g.DefineCommand(string(CommandHello), g.vimstate.helloComm, govim.NArgsZeroOrOne)
	g.DefineFunction(string(FunctionDumpPopups), []string{}, g.vimstate.dumpPopups)
	g.DefineFunction(string(FunctionShowMessagePopup), []string{}, g.vimstate.showMessagePopup)
	g.DefineFunction(string(FunctionSimpleBatch), []string{}, g.vimstate.simpleBatch)
	g.DefineFunction(string(FunctionCancelBatch), []string{}, g.vimstate.cancelBatch)
	g.DefineFunction(string(FunctionBadBatch), []string{}, g.vimstate.badBatch)
	g.DefineFunction(string(FunctionAssertFailedBatch), []string{}, g.vimstate.assertFailedBatch)
	g.DefineFunction(string(FunctionNonBatchCallInBatch), []string{}, g.vimstate.nonBatchCallInBatch)
	g.DefineFunction(string(FunctionIgnoreErrorInBatch), []string{"fail"}, g.vimstate.ignoreErrorInBatch)
}

func (v *vimstate) hello(args ...json.RawMessage) (interface{}, error) {
	return "Hello from function", nil
}

func (v *vimstate) helloComm(flags govim.CommandFlags, args ...string) error {
	msg := "Hello from command"
	if len(args) == 1 {
		msg += "; special note: " + args[0]
	}
	v.ChannelExf("echom %q", msg)
	return nil
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

func (v *vimstate) showMessagePopup(args ...json.RawMessage) (interface{}, error) {
	v.tomb.Go(func() error {
		ctx := context.Background()
		params := &protocol.ShowMessageParams{Type: protocol.Error, Message: "Something went wrong"}
		return v.ShowMessage(ctx, params)
	})
	return "", nil
}

func (v *vimstate) simpleBatch(args ...json.RawMessage) (interface{}, error) {
	v.BatchStart()
	defer v.BatchCancelIfNotEnded()
	v.BatchChannelCall("eval", "5")
	v.BatchChannelExprf("4")
	res := v.MustBatchEnd()
	return res, nil
}

func (v *vimstate) cancelBatch(args ...json.RawMessage) (interface{}, error) {
	v.BatchStart()
	defer v.BatchCancelIfNotEnded()
	return "did not run", nil
}

func (v *vimstate) badBatch(args ...json.RawMessage) (interface{}, error) {
	v.BatchStart()
	defer v.BatchCancelIfNotEnded()
	v.BatchChannelCall("execute", "throw \"failed\"")
	res := v.MustBatchEnd()
	return res, nil
}

func (v *vimstate) assertFailedBatch(args ...json.RawMessage) (interface{}, error) {
	v.BatchStart()
	defer v.BatchCancelIfNotEnded()
	v.BatchAssertChannelExprf(AssertIsZero(), "1")
	res := v.MustBatchEnd()
	return res, nil
}

func (v *vimstate) nonBatchCallInBatch(args ...json.RawMessage) (res interface{}, err error) {
	v.BatchStart()
	defer func() {
		err = recover().(error)
		v.BatchCancelIfNotEnded()
	}()
	v.ChannelExprf("1")
	v.MustBatchEnd()
	return res, err
}

func (v *vimstate) ignoreErrorInBatch(args ...json.RawMessage) (interface{}, error) {
	var fail bool
	v.Parse(args[0], &fail)
	v.BatchStart()
	defer v.BatchCancelIfNotEnded()
	assert := AssertIsErrorOrNil("E971: Property type number does not exist")
	if fail {
		var props = struct {
			Length int    `json:"length"`
			Type   string `json:"type"`
		}{
			Length: 101,
			Type:   "number",
		}
		v.BatchAssertChannelCall(assert, "prop_add", 100, 101, props)
	} else {
		v.BatchAssertChannelExprf(assert, "5")
	}
	res := v.MustBatchEnd()
	return res, nil
}
