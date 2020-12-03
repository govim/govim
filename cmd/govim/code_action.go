package main

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/types"
)

func (v *vimstate) runGoTest(flags govim.CommandFlags, args ...string) error {
	if c := v.config.ExperimentalProgressPopups; c == nil || !*c {
		opts := make(map[string]interface{})
		opts["mousemoved"] = "any"
		opts["moved"] = "any"
		opts["padding"] = []int{0, 1, 0, 1}
		opts["wrap"] = true
		opts["border"] = []int{}
		opts["highlight"] = "ErrorMsg"
		opts["line"] = 1
		opts["close"] = "click"
		v.ChannelCall("popup_create", []string{"GOVIMGoTest requires progress popups. Add this to your .vimrc:",
			" call govim#config#Set(\"ExperimentalProgressPopups\", 1)"}, opts)
		return nil
	}
	b, _, err := v.bufCursorPos()
	if err != nil {
		return fmt.Errorf("failed to get cursor position: %v", err)
	}
	start, end, err := v.rangeFromFlags(b, flags)
	if err != nil {
		return err
	}

	ca, err := v.server.CodeAction(context.Background(), &protocol.CodeActionParams{
		TextDocument: b.ToTextDocumentIdentifier(),
		Range: protocol.Range{
			Start: start.ToPosition(),
			End:   end.ToPosition(),
		},
		Context: protocol.CodeActionContext{
			Only: []protocol.CodeActionKind{protocol.GoTest},
		},
		WorkDoneProgressParams: protocol.WorkDoneProgressParams{},
	})
	if err != nil || len(ca) == 0 {
		return err
	}
	if len(ca) > 1 {
		return fmt.Errorf("got %d CodeActions, expected no more than 1", len(ca))
	}

	c := ca[0]

	token := protocol.ProgressToken(fmt.Sprintf("govim%d", rand.Uint64()))
	if _, ok := v.progressPopups[token]; ok {
		return fmt.Errorf("failed to init progress, duplicate token")
	}
	v.progressPopups[token] = &types.ProgressPopup{Initiator: types.GoTest}

	_, err = v.server.ExecuteCommand(context.Background(), &protocol.ExecuteCommandParams{
		Command:   c.Command.Command,
		Arguments: c.Command.Arguments,
		WorkDoneProgressParams: protocol.WorkDoneProgressParams{
			WorkDoneToken: token,
		},
	})

	return err
}
