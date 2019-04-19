package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kr/pretty"
	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/config"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/span"
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

	// diagnostics gives us the current diagnostics by URI
	diagnostics        map[span.URI][]protocol.Diagnostic
	diagnosticsChanged bool

	// jumpStack is akin to the Vim concept of a tagstack
	jumpStack    []protocol.Location
	jumpStackPos int

	// omnifunc calls happen in pairs (see :help complete-functions). The return value
	// from the first tells Vim where the completion starts, the return from the second
	// returns the matching words. This is by definition stateful. Hence we persist that
	// state here
	lastCompleteResults *protocol.CompletionList
}

func (v *vimstate) hello(args ...json.RawMessage) (interface{}, error) {
	return "Hello from function", nil
}

func (v *vimstate) helloComm(flags govim.CommandFlags, args ...string) error {
	v.ChannelEx(`echom "Hello from command"`)
	return nil
}

func (v *vimstate) balloonExpr(args ...json.RawMessage) (interface{}, error) {
	var vpos struct {
		BufNum int `json:"bufnum"`
		Line   int `json:"line"`
		Col    int `json:"col"`
	}
	expr := v.ChannelExpr(`{"bufnum": v:beval_bufnr, "line": v:beval_lnum, "col": v:beval_col}`)
	if err := json.Unmarshal(expr, &vpos); err != nil {
		return nil, fmt.Errorf("failed to unmarshal current mouse position info: %v", err)
	}
	b, ok := v.buffers[vpos.BufNum]
	if !ok {
		return nil, fmt.Errorf("unable to resolve buffer %v", vpos.BufNum)
	}
	pos, err := types.PointFromVim(b, vpos.Line, vpos.Col)
	if err != nil {
		return nil, fmt.Errorf("failed to determine mouse position: %v", err)
	}
	go func() {
		params := &protocol.TextDocumentPositionParams{
			TextDocument: b.ToTextDocumentIdentifier(),
			Position:     pos.ToPosition(),
		}
		hovRes, err := v.server.Hover(context.Background(), params)
		if err != nil {
			v.ChannelCall("balloon_show", fmt.Sprintf("failed to get hover details: %v", err))
		} else {
			msg := strings.TrimSpace(hovRes.Contents.Value)
			var args interface{} = msg
			if !v.isGui {
				args = strings.Split(msg, "\n")
			}
			v.ChannelCall("balloon_show", args)
		}

	}()
	return "", nil
}

func (v *vimstate) bufReadPost(args ...json.RawMessage) error {
	// Setup buffer-local mappings and settings
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

	b, err := v.currentBufferInfo(args[0])
	if err != nil {
		return err
	}
	if cb, ok := v.buffers[b.Num]; ok {
		// reload of buffer, e.v. e!
		b.Version = cb.Version + 1
	} else {
		b.Version = 0
	}
	return v.handleBufferEvent(b)
}

func (v *vimstate) bufTextChanged(args ...json.RawMessage) error {
	b, err := v.currentBufferInfo(args[0])
	if err != nil {
		return err
	}
	cb, ok := v.buffers[b.Num]
	if !ok {
		return fmt.Errorf("have not seen buffer %v (%v) - this should be impossible", b.Num, b.Name)
	}
	b.Version = cb.Version + 1
	return v.handleBufferEvent(b)
}

func (v *vimstate) handleBufferEvent(b *types.Buffer) error {
	v.buffers[b.Num] = b

	if b.Version == 0 {
		params := &protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{
				URI:     string(b.URI()),
				Version: float64(b.Version),
				Text:    string(b.Contents),
			},
		}
		err := v.server.DidOpen(context.Background(), params)
		return err
	}

	params := &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: b.ToTextDocumentIdentifier(),
			Version:                float64(b.Version),
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Text: string(b.Contents),
			},
		},
	}
	err := v.server.DidChange(context.Background(), params)
	return err
}

func (v *vimstate) formatCurrentBuffer(args ...json.RawMessage) (err error) {
	tool := v.ParseString(v.ChannelExpr(config.GlobalFormatOnSave))
	// we are an autocmd endpoint so we need to be told the current
	// buffer number via <abuf>
	currBufNr := v.ParseInt(args[0])
	b, ok := v.buffers[currBufNr]
	if !ok {
		return fmt.Errorf("failed to resolve buffer %v", currBufNr)
	}

	var edits []protocol.TextEdit

	switch config.FormatOnSave(tool) {
	case config.FormatOnSaveNone:
		return nil
	case config.FormatOnSaveGoFmt:
		params := &protocol.DocumentFormattingParams{
			TextDocument: b.ToTextDocumentIdentifier(),
		}
		edits, err = v.server.Formatting(context.Background(), params)
		if err != nil {
			return fmt.Errorf("failed to call gopls.Formatting: %v", err)
		}
	case config.FormatOnSaveGoImports:
		params := &protocol.CodeActionParams{
			TextDocument: b.ToTextDocumentIdentifier(),
		}
		actions, err := v.server.CodeAction(context.Background(), params)
		if err != nil {
			return fmt.Errorf("failed to call gopls.CodeAction: %v", err)
		}
		switch len(actions) {
		case 0:
			return nil
		case 1:
			edits = (*actions[0].Edit.Changes)[string(b.URI())]
		default:
			return fmt.Errorf("don't know how to handle %v actions", len(actions))
		}
	default:
		return fmt.Errorf("unknown format tool specified for %v: %v", config.GlobalFormatOnSave, tool)
	}

	// see :help wundo. The use of wundo! is significant. It first deletes
	// the temp file we created, but only recreates it if there is something
	// to write.  This is inherently racey... because theorectically the file
	// might in the meantime have been created by another instance of
	// govim.... We reduce that risk using the time above
	tf, err := ioutil.TempFile("", strconv.FormatInt(time.Now().UnixNano(), 10))
	if err != nil {
		return fmt.Errorf("failed to create temp undo file")
	}

	v.ChannelExf("wundo! %v", tf.Name())
	defer func() {
		if _, err := os.Stat(tf.Name()); err != nil {
			return
		}
		v.ChannelExf("silent! rundo %v", tf.Name())
		err = os.Remove(tf.Name())
	}()

	preEventIgnore := v.ParseString(v.ChannelExpr("&eventignore"))
	v.ChannelEx("set eventignore=all")
	defer v.ChannelExf("set eventignore=%v", preEventIgnore)
	v.ToggleOnViewportChange()
	defer v.ToggleOnViewportChange()
	for ie := len(edits) - 1; ie >= 0; ie-- {
		e := edits[ie]
		start, err := types.PointFromPosition(b, e.Range.Start)
		if err != nil {
			return fmt.Errorf("failed to derive start point from position: %v", err)
		}
		end, err := types.PointFromPosition(b, e.Range.End)
		if err != nil {
			return fmt.Errorf("failed to derive end point from position: %v", err)
		}

		if start.Col() != 1 || end.Col() != 1 {
			// Whether this is a delete or not, we will implement support for this later
			return fmt.Errorf("saw an edit where start col != end col (range start: %v, range end: %v start: %v, end: %v). We can't currently handle this", e.Range.Start, e.Range.End, start, end)
		}

		if start.Line() != end.Line() {
			if e.NewText != "" {
				return fmt.Errorf("saw an edit where start line != end line with replacement text %q; We can't currently handle this", e.NewText)
			}
			// This is a delete of line
			if res := v.ParseInt(v.ChannelCall("deletebufline", b.Num, start.Line(), end.Line()-1)); res != 0 {
				return fmt.Errorf("deletebufline(%v, %v, %v) failed", b.Num, start.Line(), end.Line()-1)
			}
		} else {
			// do we have anything to do?
			if e.NewText == "" {
				continue
			}
			// we are within the same line so strip the newline
			if e.NewText[len(e.NewText)-1] == '\n' {
				e.NewText = e.NewText[:len(e.NewText)-1]
			}
			repl := strings.Split(e.NewText, "\n")
			v.ChannelCall("append", start.Line()-1, repl)
		}
	}
	return nil
}

func (v *vimstate) complete(args ...json.RawMessage) (interface{}, error) {
	// Params are: findstart int, base string
	findstart := v.ParseInt(args[0]) == 1

	if findstart {
		b, pos, err := v.cursorPos()
		if err != nil {
			return nil, fmt.Errorf("failed to get current position: %v", err)
		}
		params := &protocol.CompletionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: string(b.URI()),
				},
				Position: pos.ToPosition(),
			},
		}
		res, err := v.server.Completion(context.Background(), params)
		if err != nil {
			return nil, fmt.Errorf("called to gopls.Completion failed: %v", err)
		}

		v.lastCompleteResults = res
		return pos.Col(), nil
	} else {
		var matches []completionResult
		for _, i := range v.lastCompleteResults.Items {
			matches = append(matches, completionResult{
				Abbr: i.Label,
				Word: i.TextEdit.NewText,
				Info: i.Detail,
			})
		}

		return matches, nil
	}
}

type completionResult struct {
	Abbr string `json:"abbr"`
	Word string `json:"word"`
	Info string `json:"info"`
}

func (v *vimstate) gotoDef(flags govim.CommandFlags, args ...string) error {
	cb, pos, err := v.cursorPos()
	if err != nil {
		return fmt.Errorf("failed to determine cursor position: %v", err)
	}
	params := &protocol.TextDocumentPositionParams{
		TextDocument: cb.ToTextDocumentIdentifier(),
		Position:     pos.ToPosition(),
	}
	locs, err := v.server.Definition(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to call gopls.Definition: %v\nparams were: %v", err, pretty.Sprint(params))
	}

	switch len(locs) {
	case 0:
		v.ChannelEx(`echorerr "No definition exists under cursor"`)
		return nil
	case 1:
	default:
		return fmt.Errorf("got multiple locations (%v); don't know how to handle this", len(locs))
	}

	loc := locs[0]
	v.jumpStack = append(v.jumpStack[:v.jumpStackPos], protocol.Location{
		URI: string(cb.URI()),
		Range: protocol.Range{
			Start: pos.ToPosition(),
			End:   pos.ToPosition(),
		},
	})
	v.jumpStackPos++
	return v.loadLocation(flags.Mods, loc, args...)
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
	tf := strings.TrimPrefix(loc.URI, "file://")

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

func (v *vimstate) hover(args ...json.RawMessage) (interface{}, error) {
	b, pos, err := v.cursorPos()
	if err != nil {
		return nil, fmt.Errorf("failed to get current position: %v", err)
	}
	params := &protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: string(b.URI()),
		},
		Position: pos.ToPosition(),
	}
	res, err := v.server.Hover(context.Background(), params)
	if err != nil {
		return nil, fmt.Errorf("failed to get hover details: %v", err)
	}
	return strings.TrimSpace(res.Contents.Value), nil
}

type quickfixEntry struct {
	Filename string `json:"filename"`
	Lnum     int    `json:"lnum"`
	Col      int    `json:"col"`
	Text     string `json:"text"`
}

func (v *vimstate) updateQuickfix(args ...json.RawMessage) error {
	defer func() {
		v.diagnosticsChanged = false
	}()
	if !v.diagnosticsChanged {
		return nil
	}
	var fns []span.URI
	for u := range v.diagnostics {
		fns = append(fns, u)
	}
	sort.Slice(fns, func(i, j int) bool {
		return string(fns[i]) < string(fns[j])
	})

	// TODO this will become fragile at some point
	cwd := v.ParseString(v.ChannelCall("getcwd"))

	// must be non-nil
	fixes := []quickfixEntry{}

	// now update the quickfix window based on the current diagnostics
	for _, uri := range fns {
		diags := v.diagnostics[uri]
		fn, err := uri.Filename()
		if err != nil {
			return fmt.Errorf("failed to resolve filename from URI %q: %v", uri, err)
		}
		var buf *types.Buffer
		for _, b := range v.buffers {
			if b.URI() == uri {
				buf = b
			}
		}
		if buf == nil {
			byts, err := ioutil.ReadFile(fn)
			if err != nil {
				return fmt.Errorf("failed to read contents of %v: %v", fn, err)
			}
			// create a temp buffer
			buf = &types.Buffer{
				Num:      -1,
				Name:     fn,
				Contents: byts,
			}
		}
		// make fn relative for reporting purposes
		fn, err = filepath.Rel(cwd, fn)
		if err != nil {
			return fmt.Errorf("failed to call filepath.Rel(%q, %q): %v", cwd, fn, err)
		}
		for _, d := range diags {
			p, err := types.PointFromPosition(buf, d.Range.Start)
			if err != nil {
				return fmt.Errorf("failed to resolve position: %v", err)
			}
			fixes = append(fixes, quickfixEntry{
				Filename: fn,
				Lnum:     p.Line(),
				Col:      p.Col(),
				Text:     d.Message,
			})
		}
	}
	v.ChannelCall("setqflist", fixes, "r")
	return nil
}

func (v *vimstate) deleteCurrentBuffer(args ...json.RawMessage) error {
	currBufNr := v.ParseInt(args[0])
	if _, ok := v.buffers[currBufNr]; !ok {
		return fmt.Errorf("tried to remove buffer %v; but we have no record of it", currBufNr)
	}
	delete(v.buffers, currBufNr)
	return nil
}
