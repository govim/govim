package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/types"
	"github.com/kr/pretty"
)

func (v *vimstate) gotoDef(flags govim.CommandFlags, args ...string) error {
	cb, pos, err := v.bufCursorPos()
	if err != nil {
		return fmt.Errorf("failed to determine cursor position: %v", err)
	}
	params := &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: cb.ToTextDocumentIdentifier(),
			Position:     pos.ToPosition(),
		},
	}
	locs, err := v.server.Definition(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to call gopls.Definition: %v\nparams were: %v", err, pretty.Sprint(params))
	}
	loc, err := v.handleProtocolLocations(cb, pos, locs)
	if err != nil || loc == nil {
		return err
	}
	return v.loadLocation(flags.Mods, *loc, args...)
}

func (v *vimstate) gotoTypeDef(flags govim.CommandFlags, args ...string) error {
	cb, pos, err := v.bufCursorPos()
	if err != nil {
		return fmt.Errorf("failed to determine cursor position: %v", err)
	}
	params := &protocol.TypeDefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: cb.ToTextDocumentIdentifier(),
			Position:     pos.ToPosition(),
		},
	}
	locs, err := v.server.TypeDefinition(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to call gopls.TypeDefinition: %v\nparams were: %v", err, pretty.Sprint(params))
	}
	loc, err := v.handleProtocolLocations(cb, pos, locs)
	if err != nil || loc == nil {
		return err
	}
	return v.loadLocation(flags.Mods, *loc, args...)
}

func (v *vimstate) handleProtocolLocations(b *types.Buffer, pos types.CursorPosition, locs []protocol.Location) (*protocol.Location, error) {
	switch len(locs) {
	case 0:
		v.ChannelEx(`echorerr "No definition exists under cursor"`)
		return nil, nil
	case 1:
	default:
		return nil, fmt.Errorf("got multiple locations (%v); don't know how to handle this", len(locs))
	}

	loc := locs[0]
	v.jumpStack = append(v.jumpStack[:v.jumpStackPos], protocol.Location{
		URI: protocol.DocumentURI(b.URI()),
		Range: protocol.Range{
			Start: pos.ToPosition(),
			End:   pos.ToPosition(),
		},
	})
	v.jumpStackPos++
	return &loc, nil
}

func (v *vimstate) gotoPrevDef(flags govim.CommandFlags, args ...string) error {
	if v.jumpStackPos == 0 {
		v.ChannelEx(`echom "Already at top of stack"`)
		return nil
	}
	v.jumpStackPos -= *flags.Count
	if v.jumpStackPos < 0 {
		v.jumpStackPos = 0
	}
	loc := v.jumpStack[v.jumpStackPos]

	return v.loadLocation(flags.Mods, loc, args...)
}

// args is expected to be the command args for either gotodef or gotoprevdef
func (v *vimstate) loadLocation(mods govim.CommModList, loc protocol.Location, args ...string) error {
	// We expect at most one argument that is the a string value appropriate
	// for &switchbuf. This will need parsing if supplied
	var modesStr string
	if len(args) == 1 {
		modesStr = args[0]
	} else {
		modesStr = v.ParseString(v.ChannelExpr("&switchbuf"))
	}
	var modes []govim.SwitchBufMode
	if modesStr != "" {
		pmodes, err := govim.ParseSwitchBufModes(modesStr)
		if err != nil {
			source := "from Vim setting &switchbuf"
			if len(args) == 1 {
				source = "as command argument"
			}
			return fmt.Errorf("got invalid SwitchBufMode setting %v: %q", source, modesStr)
		}
		modes = pmodes
	} else {
		modes = []govim.SwitchBufMode{govim.SwitchBufUseOpen}
	}

	modesMap := make(map[govim.SwitchBufMode]bool)
	for _, m := range modes {
		modesMap[m] = true
	}

	v.ChannelEx("normal! m'")

	vp := v.Viewport()
	tf := strings.TrimPrefix(string(loc.URI), "file://")

	bn := v.ParseInt(v.ChannelCall("bufnr", tf))
	if bn != -1 {
		if vp.Current.BufNr == bn {
			goto MovedToTargetWin
		}
		if modesMap[govim.SwitchBufUseOpen] {
			ctp := vp.Current.TabNr
			for _, w := range vp.Windows {
				if w.TabNr == ctp && w.BufNr == bn {
					v.ChannelCall("win_gotoid", w.WinID)
					goto MovedToTargetWin
				}
			}
		}
		if modesMap[govim.SwitchBufUseTag] {
			for _, w := range vp.Windows {
				if w.BufNr == bn {
					v.ChannelCall("win_gotoid", w.WinID)
					goto MovedToTargetWin
				}
			}
		}
	}
	for _, m := range modes {
		switch m {
		case govim.SwitchBufUseOpen, govim.SwitchBufUseTag:
			continue
		case govim.SwitchBufSplit:
			v.ChannelExf("%v split %v", mods, tf)
		case govim.SwitchBufVsplit:
			v.ChannelExf("%v vsplit %v", mods, tf)
		case govim.SwitchBufNewTab:
			v.ChannelExf("%v tabnew %v", mods, tf)
		}
		goto MovedToTargetWin
	}

	// I _think_ the default behaviour at this point is to use the
	// current window, i.e. simply edit
	v.ChannelExf("%v edit %v", mods, tf)

MovedToTargetWin:

	// now we _must_ have a valid buffer
	bn = v.ParseInt(v.ChannelCall("bufnr", tf))
	if bn == -1 {
		return fmt.Errorf("should have a valid buffer number by this point; we don't")
	}
	nb, ok := v.buffers[bn]
	if !ok {
		return fmt.Errorf("should have resolved a buffer; we didn't")
	}
	newPos, err := types.PointFromPosition(nb, loc.Range.Start)
	if err != nil {
		return fmt.Errorf("failed to derive point from position: %v", err)
	}
	v.ChannelCall("cursor", newPos.Line(), newPos.Col())
	v.ChannelEx("normal! zz")

	return nil
}
