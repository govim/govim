package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/types"
)

type callHierarchy struct {
	lines    []*callHierarchyEntry
	outgoing bool
	bufNr    int
}

type callHierarchyEntry struct {
	item     protocol.CallHierarchyItem
	parent   *protocol.CallHierarchyItem
	calls    []protocol.Range
	indent   int
	expanded bool
}

func (v *vimstate) redrawCallHierarchy(clean bool) error {
	ch := v.callhierarchy
	v.BatchStart()
	v.BatchChannelCall("setbufvar", ch.bufNr, "&modifiable", 1)
	if clean {
		v.BatchChannelCall("deletebufline", ch.bufNr, 1, '$')
	}
	for i := 0; i < len(ch.lines); i++ {
		var prefix string
		switch {
		case i == 0 && ch.outgoing:
			prefix = "Calls from"
		case i == 0 && !ch.outgoing:
			prefix = "Calls to"
		case ch.lines[i].expanded:
			prefix = "-"
		default:
			prefix = "+"
		}
		var count string
		if l := len(ch.lines[i].calls); l > 0 { // root node won't have references
			count = fmt.Sprintf("[%d]", l)
		}
		indent := strings.Repeat(" ", ch.lines[i].indent*2)
		v.BatchChannelCall("setbufline", ch.bufNr, i+1 /* 1-indexed */, fmt.Sprintf(" %s%s %s %s %s",
			indent,
			prefix,
			ch.lines[i].item.Name,
			ch.lines[i].item.Detail,
			count,
		))
	}
	v.BatchChannelCall("setbufvar", ch.bufNr, "&modifiable", 0)
	v.MustBatchEnd()
	return nil
}

func (v *vimstate) callHierarchy(flags govim.CommandFlags, args ...string) error {
	ch := v.callhierarchy
	// TODO: probably want to check the explicit ones here..
	if len(args) == 0 {
		return fmt.Errorf("call with either 'in', 'out' or 'goto' as argument")
	}

	bufNr := v.ParseInt(v.ChannelCall("bufadd", "govim-callhierarchy"))
	ch.bufNr = bufNr
	v.ChannelExf("silent call bufload(%d)", bufNr) // must load buffer before setting bufvars & adding content
	pos, err := v.cursorPos()
	if err != nil {
		return err
	}

	var clean bool
	var hierarchyCursorLine int = -1 // line in call hierarchy buffer

	// Called from outside the current call hierarchy so we need to create a fresh one
	if pos.BufNr != bufNr {
		b, ok := v.buffers[pos.BufNr]
		if !ok {
			return fmt.Errorf("buffer not tracked by govim")
		}

		ch.lines = make([]*callHierarchyEntry, 1)
		params := &protocol.CallHierarchyPrepareParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI(b.URI()),
				},
				Position: pos.ToPosition(),
			},
		}
		chs, err := v.server.PrepareCallHierarchy(context.Background(), params)
		if err != nil {
			return fmt.Errorf("call to gopls.PrepareCallHierarchy failed: %v", err)
		}

		if len(chs) == 0 {
			return nil
		} else if len(chs) > 1 {
			return fmt.Errorf("got more than one CallHierarchyItem in response, can't handle")
		}

		if winid := v.ParseInt(v.ChannelCall("bufwinid", bufNr)); winid == -1 {
			v.BatchStart()
			v.BatchChannelCall("setbufvar", bufNr, "&buftype", "nofile")
			v.BatchChannelCall("setbufvar", bufNr, "&swapfile", 0)
			v.BatchChannelCall("setbufvar", bufNr, "&buflisted", 0)
			v.BatchChannelCall("setbufvar", bufNr, "&modifiable", 0)
			v.MustBatchEnd()
			v.ChannelExf("10split govim-callhierarchy")
			v.ChannelCall("setbufvar", bufNr, "&cursorline", 1) // TODO: why can't this be called in the batch?
			v.ChannelExf(`nnoremap <silent> <buffer> <Leader>i :GOVIMCallHierarchy in<CR>`)
			v.ChannelExf(`nnoremap <silent> <buffer> <Leader>o :GOVIMCallHierarchy out<CR>`)
			v.ChannelExf(`nnoremap <silent> <buffer> <Leader>p :GOVIMCallHierarchy goto<CR>`)
		} else {
			v.ChannelCall("win_gotoid", winid)
		}

		hierarchyCursorLine = 0
		clean = true
		ch.lines[hierarchyCursorLine] = &callHierarchyEntry{chs[0], nil, nil, 0, false}
	}

	if hierarchyCursorLine == -1 {
		// TODO: refactor? line must be fetched after call hierarchy buffer is created above.
		hierarchyCursorLine = v.ParseInt(v.ChannelExpr("line('.')")) - 1 // zero indexed
	}

	current := ch.lines[hierarchyCursorLine]

	type newEntry struct {
		item  protocol.CallHierarchyItem
		calls []protocol.Range
	}
	var newEntries []newEntry
	switch args[0] {
	case "in":
		in, err := v.server.IncomingCalls(context.Background(), &protocol.CallHierarchyIncomingCallsParams{
			Item: current.item,
		})
		if err != nil {
			return err
		}
		if ch.outgoing { // was showing outgoing, so clean the tree
			ch.lines = []*callHierarchyEntry{{item: current.item}}
			hierarchyCursorLine = 0
			clean = true
		}
		ch.outgoing = false
		for i := range in {
			newEntries = append(newEntries, newEntry{in[i].From, in[i].FromRanges})
		}
	case "out":
		out, err := v.server.OutgoingCalls(context.Background(), &protocol.CallHierarchyOutgoingCallsParams{
			Item: current.item,
		})
		if err != nil {
			return err
		}
		if !ch.outgoing { // was showing incoming, so clean the tree
			ch.lines = []*callHierarchyEntry{{item: current.item}}
			hierarchyCursorLine = 0
			clean = true
		}
		ch.outgoing = true
		for i := range out {
			newEntries = append(newEntries, newEntry{out[i].To, out[i].FromRanges})
		}
	case "goto":
		// TODO: check if buffer is already open and jump to that window
		// v.ChannelExf("b%d", p.Buffer().Num)
		// else just jump to previous and open there:
		v.ChannelEx("wincmd p") // Jump to previous window
		v.ChannelExf("e %s", current.item.URI.SpanURI().Filename())
		var b *types.Buffer
		for _, b = range v.buffers {
			if b.URI() == span.URI(current.item.URI) {
				break
			}
		}
		p, err := types.PointFromPosition(b, current.item.Range.Start)
		if err != nil {
			return err
		}
		v.ChannelCall("cursor", p.Line(), p.Col())
		return nil
	default:
		return fmt.Errorf(`argument must be either "out", "in" or "goto"`)
	}

	// TODO: order by something else?
	sort.Slice(newEntries, func(i, j int) bool {
		return newEntries[i].item.Name < newEntries[j].item.Name
	})

	cursorIndent := ch.lines[hierarchyCursorLine].indent

	// TODO: check this earlier
	if ch.lines[hierarchyCursorLine].expanded {
		// already expanded, skip
		return nil
	}
	ch.lines[hierarchyCursorLine].expanded = true

	defer v.redrawCallHierarchy(clean)

	if len(newEntries) == 0 {
		return nil
	}

	// insert new entries
	tmp := make([]*callHierarchyEntry, len(ch.lines)+len(newEntries))
	copy(tmp, ch.lines[:hierarchyCursorLine+1])
	for i := range newEntries {
		tmp[hierarchyCursorLine+1+i] = &callHierarchyEntry{newEntries[i].item, &ch.lines[hierarchyCursorLine].item, newEntries[i].calls, cursorIndent + 1, false}
	}
	copy(tmp[hierarchyCursorLine+len(newEntries)+1:], ch.lines[hierarchyCursorLine+1:])
	ch.lines = tmp

	return nil
}

// TODO: must be called from within the call hierarchy buffer, fix that
func (v *vimstate) updateCallhierarchyHighlight() error {
	ch := v.callhierarchy
	line := v.ParseInt(v.ChannelExpr("line('.')")) - 1 // 0 indexed
	if line >= len(ch.lines) {
		return nil
	}
	entry := ch.lines[line]

	children := make(map[*types.Buffer][]protocol.Range)
	l := line - 1
	for {
		l++
		e := ch.lines[l]
		if e.indent != entry.indent+1 {
			break
		}
		var buf *types.Buffer
		for _, b := range v.buffers {
			if b.URI() == span.URI(e.item.URI) {
				buf = b
			}
		}

		// can't place highlights in buffer that isn't loaded
		if buf == nil || !buf.Loaded {
			continue
		}
		if _, ok := children[buf]; !ok {
			children[buf] = make([]protocol.Range, 0, 1)
		}
		children[buf] = append(children[buf], e.item.Range)
	}

	// TODO: rewrite
	var buf *types.Buffer
	var fromBuf *types.Buffer
	for _, b := range v.buffers {
		if b.URI() == span.URI(entry.item.URI) {
			buf = b
		}

		if v.callhierarchy.outgoing {
			if entry.parent != nil && b.URI() == span.URI(entry.parent.URI) {
				fromBuf = b
			}
		}
	}
	if !v.callhierarchy.outgoing {
		fromBuf = buf
	}

	return v.callHierarchyHighlight(buf, entry.item.Range, children, entry.calls, fromBuf)
}
