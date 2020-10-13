package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/stringfns"
)

func (v *vimstate) stringfns(flags govim.CommandFlags, args ...string) error {
	var transFns []string
	for _, fp := range args {
		if _, ok := stringfns.Functions[fp]; !ok {
			return fmt.Errorf("failed to resolve transformation function %q", fp)
		}
		transFns = append(transFns, fp)
	}

	b, _, err := v.bufCursorPos()
	if err != nil {
		return err
	}

	start, end, err := v.rangeFromFlags(b, flags)
	if err != nil {
		return err
	}

	newText := string(b.Contents()[start.Offset():end.Offset()])
	for _, fp := range transFns {
		fn := stringfns.Functions[fp]
		newText, err = fn(string(newText))
		if err != nil {
			return fmt.Errorf("failed to apply ")
		}
	}

	edit := protocol.TextEdit{
		Range: protocol.Range{
			Start: start.ToPosition(),
			End:   end.ToPosition(),
		},
		NewText: newText,
	}
	return v.applyProtocolTextEdits(b, []protocol.TextEdit{edit})
}

func (v *vimstate) stringfncomplete(args ...json.RawMessage) (interface{}, error) {
	lead := v.ParseString(args[0])
	var results []string
	for k := range stringfns.Functions {
		if strings.HasPrefix(k, lead) {
			results = append(results, k)
		}
	}
	sort.Strings(results)
	return results, nil
}
