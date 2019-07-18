package main

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/config"
)

// This file contains config that would otherwise be in the
// github.com/myitcv/govim/cmd/govim/config, but for the fact that these are
// only definitions we need for the purposes of testing

const (
	CommandHello config.Command = "Hello"
)

const (
	FunctionHello      config.Function = "Hello"
	FunctionDumpPopups config.Function = config.InternalFunctionPrefix + "DumpPopups"
)

func (g *govimplugin) InitTestAPI() {
	if !exposeTestAPI {
		return
	}

	g.DefineFunction(string(FunctionHello), []string{}, g.vimstate.hello)
	g.DefineCommand(string(CommandHello), g.vimstate.helloComm)
	g.DefineFunction(string(FunctionDumpPopups), []string{}, g.vimstate.dumpPopups)
}

func (v *vimstate) hello(args ...json.RawMessage) (interface{}, error) {
	return "Hello from function", nil
}

func (v *vimstate) helloComm(flags govim.CommandFlags, args ...string) error {
	v.ChannelEx(`echom "Hello from command"`)
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
