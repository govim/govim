package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
)

func (v *vimstate) suggestFixes(flags govim.CommandFlags, args ...string) error {
	switch len(v.suggestedFixesPopups) {
	case 0: // No popup open, create new ones
	case 1: // Close the only popup
		var popupID int
		for popupID = range v.suggestedFixesPopups {
		}
		v.ChannelCall("popup_close", popupID)
		delete(v.suggestedFixesPopups, popupID)
	default: // Cycle to next popup
		var popups []int
		for pid := range v.suggestedFixesPopups {
			popups = append(popups, pid)
		}

		sort.Ints(popups)

		var offset int
		switch {
		case len(args) < 1:
			return nil // Suggested fixes triggered with a popup open?
		case args[0] == "next":
			offset = 1
		case args[0] == "prev":
			offset = len(popups) - 1
		default:
			return fmt.Errorf("invalid cycle direction argument to suggestedFixes")
		}

		type popupPos struct {
			Visible int `json:"visible"`
		}

		for i, pid := range popups {
			var pp popupPos
			v.Parse(v.ChannelCall("popup_getpos", pid), &pp)
			if pp.Visible == 1 {
				v.ChannelCall("popup_hide", pid)
				v.ChannelCall("popup_show", popups[(i+offset)%len(popups)])
				return nil
			}
		}

		// We should never reach this line since there are more than one
		// open popup, and one of the popups is always supposed to be visible
		return fmt.Errorf("failed to find a visible popup")
	}

	cb, pos, err := v.bufCursorPos()
	if err != nil {
		return fmt.Errorf("failed to determine cursor position: %v", err)
	}
	start := pos.ToPosition()
	end := pos.ToPosition()
	textDoc := cb.ToTextDocumentIdentifier()

	var coveredDiags []protocol.Diagnostic
	v.diagnosticsChangedLock.Lock()
	if diags, ok := v.rawDiagnostics[cb.URI()]; ok {
		for _, d := range diags.Diagnostics {
			// TODO: should we go for "current line" as default?
			if int(pos.ToPosition().Line) >= int(d.Range.Start.Line) &&
				int(pos.ToPosition().Line) <= int(d.Range.End.Line) {
				coveredDiags = append(coveredDiags, d)
			}
		}
	}
	v.diagnosticsChangedLock.Unlock()

	params := &protocol.CodeActionParams{
		TextDocument: textDoc,
		Range:        protocol.Range{Start: start, End: end},
		Context: protocol.CodeActionContext{
			Diagnostics: coveredDiags,
			Only:        []protocol.CodeActionKind{protocol.QuickFix},
		},
	}
	codeActions, err := v.server.CodeAction(context.Background(), params)
	if err != nil {
		return fmt.Errorf("codeAction failed: %v", err)
	}

	resolvableDiags := diagSuggestions(codeActions)
	for i := range resolvableDiags {
		suggestions := resolvableDiags[i].suggestions
		opts := make(map[string]interface{})
		opts["line"] = "cursor+1"
		opts["col"] = "cursor"
		opts["drag"] = 1
		opts["mapping"] = 0
		opts["cursorline"] = 1
		opts["filter"] = "GOVIM_internal_SuggestedFixesFilter"
		opts["title"] = resolvableDiags[i].title
		opts["callback"] = "GOVIM" + config.FunctionPopupSelection
		if i > 0 {
			opts["hidden"] = 1
		}

		alts := make([]string, len(suggestions))
		edits := make([]protocol.WorkspaceEdit, len(suggestions))
		for j := range suggestions {
			alts[j] = suggestions[j].msg
			edits[j] = suggestions[j].edit
		}

		if len(resolvableDiags) > 1 {
			opts["title"] = fmt.Sprintf("%s [%d/%d]", opts["title"], i+1, len(resolvableDiags))
		}

		popupID := v.ParseInt(v.ChannelCall("popup_create", alts, opts))
		v.suggestedFixesPopups[popupID] = edits
	}

	return nil
}

type resolvableDiag struct {
	title       string
	suggestions []suggestion
}

type suggestion struct {
	msg  string
	edit protocol.WorkspaceEdit
}

func diagSuggestions(codeActions []protocol.CodeAction) []resolvableDiag {
	// The CodeAction response can contain several responses that each
	// one might solve one or several diagnostics.
	// In gopls, diagnostics are currently keyed by "Message" + "Range"
	// so we transpose the responses into diagnostics to be able to
	// differentiate between "two different diagnostics" and "one diag
	// diagnostic with two suggested fixes".

	type diagKey struct {
		msg string
		r   protocol.Range
	}

	resolvableDiags := make(map[diagKey][]suggestion)
	for _, ca := range codeActions {
		if ca.Kind != protocol.QuickFix {
			continue
		}
		for i := range ca.Diagnostics {
			k := diagKey{ca.Diagnostics[i].Message, ca.Diagnostics[i].Range}
			if _, exist := resolvableDiags[k]; !exist {
				resolvableDiags[k] = make([]suggestion, 0)
			}
			resolvableDiags[k] = append(resolvableDiags[k], suggestion{ca.Title, ca.Edit})
		}
	}

	var out []resolvableDiag
	for k, v := range resolvableDiags {
		out = append(out, resolvableDiag{
			title:       k.msg,
			suggestions: v,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].title < out[j].title
	})

	return out
}
