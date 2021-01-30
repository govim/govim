package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

	// popupWinID is the id of the window currently being used for a hover-based popup
	popupWinID int

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
	suggestedFixesPopups map[int][]suggestedFix

	// working directory (when govim was started)
	// TODO: handle changes to current working directory during runtime
	workingDirectory string

	// currentReferences is the range of each LSP documentHighlights under the cursor
	// It is used to avoid updating the text property when the cursor is moved within the
	// existing highlights.
	currentReferences []*types.Range

	// highlightingReferences indicates the user has explicitly called
	// CommandHighlightReferences. When set, those highlights are only removed
	// through an explicit call to CommandClearReferencesHighlights or via a
	// change to any file (because we can't know without requerying gopls
	// whether the highlights are still correct/accurate/etc)
	highlightingReferences bool

	// progressPopup is a map of ongoing progresses. Before receiving the first
	// progress, the value is nil. Added popups should use the internal
	// ProgressClosed function as callback to ensure that entities are removed
	// from this map when the popup closes.
	progressPopups map[protocol.ProgressToken]*types.ProgressPopup

	// lastProgressText points to the text in the most recently created progress
	// popup.
	lastProgressText *strings.Builder

	// vimgrepPendingBufs contain buffers read during a vimgrep quickfix command,
	// keyed by buffer number.
	// The purpose is to avoid sending DidOpen/DidClose notifications to gopls
	// for all files which contained no pattern match.
	// Vim will reuse the buffer number as long as the searched file didn't contain
	// a pattern match. See discussion at https://groups.google.com/g/vim_dev/c/qpu2O8cvprY.
	//
	// A nil value is used to indicate that there is no ongoing vimgrep.
	// When vimgrep is done, these buffers are added to govim.
	vimgrepPendingBufs map[int]*types.Buffer
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
			if err := v.updateReferenceHighlight(true); err != nil {
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

	var fixes []suggestedFix
	var ok bool
	if fixes, ok = v.suggestedFixesPopups[popupID]; !ok {
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

	fix := fixes[selection-1]

	// Edits should be applied before any Command according to LSP 3.16.
	if len(fix.edit.DocumentChanges) > 0 {
		if err := v.applyMultiBufTextedits(nil, fix.edit.DocumentChanges); err != nil {
			return nil, err
		}
	}

	if fix.command != nil {
		editsCh := make(chan applyEditCall)
		v.govimplugin.applyEditsLock.Lock()
		v.govimplugin.applyEditsCh = editsCh
		v.govimplugin.applyEditsLock.Unlock()
		done := make(chan struct{})

		var ecErr error
		v.tomb.Go(func() error {
			_, ecErr = v.server.ExecuteCommand(context.Background(),
				&protocol.ExecuteCommandParams{
					Command:   fix.command.Command,
					Arguments: fix.command.Arguments,
				})

			v.govimplugin.applyEditsLock.Lock()
			v.govimplugin.applyEditsCh = nil
			v.govimplugin.applyEditsLock.Unlock()
			close(done)
			return nil
		})

		for {
			select {
			case <-done:
				if ecErr != nil {
					return nil, fmt.Errorf("executeCommand failed: %v", ecErr)
				}
				return nil, nil
			case c := <-editsCh:
				res, err := v.applyWorkspaceEdit(c.params)
				c.responseCh <- applyEditResponse{res, err}
			}
		}
	}
	return nil, nil
}

func (v *vimstate) progressClosed(args ...json.RawMessage) (interface{}, error) {
	var popupID int
	v.Parse(args[0], &popupID)

	var toDelete protocol.ProgressToken
	for token, popup := range v.progressPopups {
		if popup.ID == popupID {
			if toDelete != nil {
				return nil, fmt.Errorf("found multiple popups with same ID, can't handle")
			}
			toDelete = token
		}
	}

	delete(v.progressPopups, toDelete)
	v.rearrangeProgressPopups()

	return nil, nil
}

func (v *vimstate) setUserBusy(args ...json.RawMessage) (interface{}, error) {
	v.userBusy = v.ParseInt(args[0]) != 0
	pos, err := v.parseCursorPos(args[1])
	if err != nil {
		return nil, fmt.Errorf("failed to get cursor position: %v", err)
	}
	if v.userBusy {
		// We are now busy
		return nil, v.removeReferenceHighlight(&pos)
	}
	// We are now idle
	if err := v.updateReferenceHighlightAtCursorPosition(false, pos); err != nil {
		return nil, err
	}
	if err := v.handleDiagnosticsChanged(); err != nil {
		return nil, err
	}
	return nil, nil
}
