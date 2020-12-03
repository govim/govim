package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/types"
)

const progressMaxHeight = 10

func (v *vimstate) handleProgress(popup *types.ProgressPopup, kind, title, message string) error {
	popup.Text.WriteString(message + "\n")
	lines := strings.Split(popup.Text.String(), "\n")

	// To achive automatic scrolling in popups we use the "firstline" option to specify the
	// first line to show.
	firstline := len(lines) - progressMaxHeight
	if firstline < 1 {
		firstline = 1
	}
	switch kind {
	case "begin":
		w := v.ParseInt(v.ChannelCall("winwidth", 0))

		popup.LinePos = 1
		opts := map[string]interface{}{
			"pos":       "topright",
			"line":      popup.LinePos,
			"col":       w,
			"padding":   []int{0, 1, 0, 1},
			"wrap":      false,
			"close":     "click",
			"title":     title,
			"zindex":    300, // same as popup_notification()
			"mapping":   0,
			"border":    []string{},
			"minwidth":  40,
			"maxwidth":  40,
			"maxheight": progressMaxHeight,
			"firstline": firstline,
			"scrollbar": false,
			"callback":  "GOVIM" + config.FunctionProgressClosed,
		}
		popup.ID = v.ParseInt(v.ChannelCall("popup_create", lines, opts))
		v.lastProgressText = &popup.Text
	case "report":
		opts := map[string]interface{}{
			"firstline": firstline,
		}
		v.BatchStart()
		v.BatchChannelCall("popup_settext", popup.ID, lines)
		v.BatchChannelCall("popup_setoptions", popup.ID, opts)
		v.MustBatchEnd()
	case "end":
		opts := map[string]interface{}{
			"time":      3000, // close after 3 seconds, as popup_notification()
			"firstline": firstline,
		}
		if popup.Initiator == types.GoTest {
			// gopls could run several go test invocations within the same progress
			// so we must parse the entire output, otherwise we could have relied on
			// deltas only and update in both "report" and "end".
			if hl := v.testOutputToHighlight(popup.Text.String()); hl != "" {
				opts["highlight"] = hl
				opts["borderhighlight"] = []string{string(hl)}
			}
		}
		v.BatchStart()
		v.BatchChannelCall("popup_settext", popup.ID, lines)
		v.BatchChannelCall("popup_setoptions", popup.ID, opts)
		v.MustBatchEnd()
	}

	v.rearrangeProgressPopups()
	return nil
}

func (v *vimstate) testOutputToHighlight(text string) config.Highlight {
	var hl config.Highlight = ""
	for _, l := range strings.Split(text, "\n") {
		switch l {
		case "FAIL":
			return config.HighlightGoTestFail
		case "PASS":
			hl = config.HighlightGoTestPass
		}
	}
	return hl
}

// rearrangeProgressPopups will move progress popups so that they are sorted by
// popup ID and thus creation time.
func (v *vimstate) rearrangeProgressPopups() {
	popups := make([]*types.ProgressPopup, 0, len(v.progressPopups))
	for _, p := range v.progressPopups {
		if p != nil {
			popups = append(popups, p)
		}
	}
	if len(popups) == 0 {
		return
	}

	sort.Slice(popups, func(i, j int) bool {
		return popups[i].ID < popups[j].ID
	})

	v.BatchStart()
	linePos := 1
	for i := range popups {
		if popups[i].LinePos != linePos {
			popups[i].LinePos = linePos
			v.BatchChannelCall("popup_setoptions",
				popups[i].ID,
				map[string]interface{}{"line": popups[i].LinePos},
			)
		}
		lines := len(strings.Split(popups[i].Text.String(), "\n"))
		if lines > progressMaxHeight {
			lines = progressMaxHeight
		}
		linePos += lines + 2 // lines + top & bottom border
	}
	v.MustBatchEnd()
}

func (v *vimstate) openLastProgress(flags govim.CommandFlags, args ...string) error {
	if v.lastProgressText == nil {
		return nil
	}

	bufName := fmt.Sprintf("gopls-progress-%s", time.Now().Format("20060102_150405000"))
	bufNr := v.ParseInt(v.ChannelCall("bufadd", bufName))
	v.ChannelExf("silent call bufload(%d)", bufNr)
	v.BatchStart()
	v.BatchChannelCall("setbufvar", bufNr, "&buftype", "nofile")
	v.BatchChannelCall("setbufvar", bufNr, "&swapfile", 0)
	v.BatchChannelCall("setbufvar", bufNr, "&buflisted", 1)
	v.BatchChannelCall("setbufline", bufNr, 1, strings.Split(v.lastProgressText.String(), "\n"))
	v.MustBatchEnd()
	if open := v.config.OpenLastProgressWith; open != nil && *open != "" {
		v.ChannelExf("%s %s", *open, bufName)
	}
	return nil
}
