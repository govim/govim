package main

import (
	"encoding/json"
	"fmt"

	"github.com/myitcv/govim/cmd/govim/config"
)

// ftplugin is where govim defines filetype-based mappings, settings etc
func (v *vimstate) ftplugin(args ...json.RawMessage) (interface{}, error) {
	amatch := v.ParseString(args[0])
	switch amatch {
	case "go":
		v.ChannelExf("setlocal balloonexpr=%v%v()", v.Driver.Prefix(), config.FunctionBalloonExpr)
		v.ChannelExf("setlocal omnifunc=%v%v", v.Driver.Prefix(), config.FunctionComplete)
		v.ChannelExf("nnoremap <buffer> <silent> <C-]> :%v%v<cr>", v.Driver.Prefix(), config.CommandGoToDef)
		v.ChannelExf("nnoremap <buffer> <silent> gd :%v%v<cr>", v.Driver.Prefix(), config.CommandGoToDef)
		v.ChannelExf("nnoremap <buffer> <silent> <C-]> :%v%v<cr>", v.Driver.Prefix(), config.CommandGoToDef)
		v.ChannelExf("nnoremap <buffer> <silent> <C-LeftMouse> <LeftMouse>:%v%v<cr>", v.Driver.Prefix(), config.CommandGoToDef)
		v.ChannelExf("nnoremap <buffer> <silent> g<LeftMouse> <LeftMouse>:%v%v<cr>", v.Driver.Prefix(), config.CommandGoToDef)
		v.ChannelExf("nnoremap <buffer> <silent> <C-t> :%v%v<cr>", v.Driver.Prefix(), config.CommandGoToPrevDef)
		v.ChannelExf("nnoremap <buffer> <silent> <C-RightMouse> <RightMouse>:%v%v<cr>", v.Driver.Prefix(), config.CommandGoToDef)
		v.ChannelExf("nnoremap <buffer> <silent> g<RightMouse> <RightMouse>:%v%v<cr>", v.Driver.Prefix(), config.CommandGoToDef)
	default:
		return nil, fmt.Errorf("don't yet know how to handle filetype of %v", amatch)
	}
	return nil, nil
}
