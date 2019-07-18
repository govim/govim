package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/config"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/types"
)

var stringFns = map[string]interface{}{
	"strconv.Quote":               strconv.Quote,
	"strconv.Unquote":             strconv.Unquote,
	"regexp.QuoteMeta":            regexp.QuoteMeta,
	"crypto/sha256.Sum256":        sha256Sum,
	"encoding/hex.EncodeToString": hexEncode,
}

func (v *vimstate) stringfns(flags govim.CommandFlags, args ...string) error {
	var transFns []interface{}
	for _, fp := range args {
		fn, ok := stringFns[fp]
		if !ok {
			return fmt.Errorf("failed to resolve transformation function %q", fp)
		}
		transFns = append(transFns, fn)
	}
	var err error
	var start, end types.Point
	var b *types.Buffer
	switch *flags.Range {
	case 2:
		// we have a range
		var pos struct {
			BuffNr int    `json:"buffnr"`
			Mode   string `json:"mode"`
			Start  []int  `json:"start"`
			End    []int  `json:"end"`
		}
		v.Parse(v.ChannelExpr(`{"buffnr": bufnr(""), "mode": visualmode(), "start": getpos("'<"), "end": getpos("'>")}`), &pos)

		if pos.Mode == "\x16" {
			return fmt.Errorf("cannot use %v in visual block mode", config.CommandStringFn)
		}

		var ok bool
		b, ok = v.buffers[pos.BuffNr]
		if !ok {
			return fmt.Errorf("failed to derive buffer")
		}

		start, err = types.PointFromVim(b, pos.Start[1], pos.Start[2])
		if err != nil {
			return fmt.Errorf("failed to get start position of range: %v", err)
		}
		if pos.Mode == "V" {
			// we have already parsed start so we can mutate here
			pos.End = pos.Start
			pos.End[1]++
		}
		end, err = types.PointFromVim(b, pos.End[1], pos.End[2])
		if err != nil {
			return fmt.Errorf("failed to get end position of range: %v", err)
		}
		if pos.Mode == "v" {
			// we need to move past the end of the selection
			rem := b.Contents()[end.Offset():]
			if len(rem) > 0 {
				_, adj := utf8.DecodeRune(rem)
				end, err = types.PointFromVim(b, pos.End[1], pos.End[2]+adj)
				if err != nil {
					return fmt.Errorf("failed to get adjusted end position: %v", err)
				}
			}
		}
	case 0:
		// current line
		b, _, err = v.cursorPos()
		if err != nil {
			return fmt.Errorf("failed to get cursor position for line range")
		}
		start, err = types.PointFromVim(b, *flags.Line1, 1)
		if err != nil {
			return fmt.Errorf("failed to get start position of range: %v", err)
		}
		end, err = types.PointFromVim(b, *flags.Line1+1, 1)
		if err != nil {
			return fmt.Errorf("failed to get end position of range: %v", err)
		}
	default:
		return fmt.Errorf("unknown range indicator %v", *flags.Range)
	}

	endOffset := end.Offset()
	if *flags.Range == 0 {
		endOffset--
	}
	newText := string(b.Contents()[start.Offset():endOffset])
	for fp, fn := range transFns {
		switch fn := fn.(type) {
		case func(string) string:
			newText = fn(string(newText))
		default:
			return fmt.Errorf("do not know how to handle transformation function %q of type %T", fp, fn)
		}
	}
	if *flags.Range == 0 {
		newText += "\n"
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
	for k := range stringFns {
		if strings.HasPrefix(k, lead) {
			results = append(results, k)
		}
	}
	sort.Strings(results)
	return results, nil
}

func sha256Sum(s string) string {
	v := sha256.Sum256([]byte(s))
	return string(v[:])
}

func hexEncode(s string) string {
	return hex.EncodeToString([]byte(s))
}
