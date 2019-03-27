package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kr/pretty"
	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/span"
	"github.com/russross/blackfriday/v2"
)

func (d *driver) hello(args ...json.RawMessage) (interface{}, error) {
	return "Hello from function", nil
}

func (d *driver) helloComm(flags govim.CommandFlags, args ...string) error {
	d.ChannelEx(`echom "Hello from command"`)
	return nil
}

func (d *driver) balloonExpr(args ...json.RawMessage) (interface{}, error) {
	pos, err := d.mousePos()
	if err != nil {
		return nil, fmt.Errorf("failed to get current position: %v", err)
	}
	go func() {
		params := &protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: string(span.FileURI(pos.Filename)),
			},
			Position: protocol.Position{
				Line:      float64(pos.Line - 1),
				Character: float64(pos.Col - 1),
			},
		}
		d.Logf("calling Hover: %v", pretty.Sprint(params))
		hovRes, err := d.server.Hover(context.Background(), params)
		if err != nil {
			d.ChannelCall("balloon_show", fmt.Sprintf("failed to get hover details: %v", err))
		} else {
			d.Logf("got Hover results: %q", hovRes.Contents.Value)
			md := []byte(hovRes.Contents.Value)
			plain := string(blackfriday.Run(md, blackfriday.WithRenderer(plainMarkdown{})))
			plain = strings.TrimSpace(plain)
			d.ChannelCall("balloon_show", strings.Split(plain, "\n"))
		}

	}()
	return "", nil
}
