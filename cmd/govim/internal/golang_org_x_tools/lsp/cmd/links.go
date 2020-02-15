// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/tool"
	errors "golang.org/x/xerrors"
)

// links implements the links verb for gopls.
type links struct {
	JSON bool `flag:"json" help:"emit document links in JSON format"`

	app *Application
}

func (l *links) Name() string      { return "links" }
func (l *links) Usage() string     { return "<filename>" }
func (l *links) ShortHelp() string { return "list links in a file" }
func (l *links) DetailedHelp(f *flag.FlagSet) {
	fmt.Fprintf(f.Output(), `
Example: list links contained within a file:

  $ gopls links internal/lsp/cmd/check.go

gopls links flags are:
`)
	f.PrintDefaults()
}

// Run finds all the links within a document
// - if -json is specified, outputs location range and uri
// - otherwise, prints the a list of unique links
func (l *links) Run(ctx context.Context, args ...string) error {
	if len(args) != 1 {
		return tool.CommandLineErrorf("links expects 1 argument")
	}
	conn, err := l.app.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.terminate(ctx)

	from := span.Parse(args[0])
	uri := from.URI()
	file := conn.AddFile(ctx, uri)
	if file.err != nil {
		return file.err
	}
	results, err := conn.DocumentLink(ctx, &protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.URIFromSpanURI(uri),
		},
	})
	if err != nil {
		return errors.Errorf("%v: %v", from, err)
	}
	if l.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "\t")
		return enc.Encode(results)
	}
	for _, v := range results {
		fmt.Println(v.Target)
	}
	return nil
}
