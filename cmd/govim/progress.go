package main

import (
	"sort"
	"strings"

	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/types"
)

func (v *vimstate) handleProgress(popup *types.ProgressPopup, kind, title, message string) error {
	text := strings.Split(message, "\n")
	switch kind {
	case "begin":
		w := v.ParseInt(v.ChannelCall("winwidth", 0))

		popup.Text = text
		popup.LinePos = 1
		opts := map[string]interface{}{
			"pos":      "topright",
			"line":     popup.LinePos,
			"col":      w,
			"padding":  []int{0, 1, 0, 1},
			"wrap":     false,
			"close":    "click",
			"title":    title,
			"zindex":   300, // same as popup_notification()
			"mapping":  0,
			"border":   []string{},
			"minwidth": 20, // same as popup_notification()
			"callback": "GOVIM" + config.FunctionProgressClosed,
		}
		popup.ID = v.ParseInt(v.ChannelCall("popup_create", popup.Text, opts))
	case "report":
		popup.Text = append(popup.Text, text...)
		v.ChannelCall("popup_settext", popup.ID, popup.Text)
	case "end":
		popup.Text = append(popup.Text, text...)
		v.BatchStart()
		v.BatchChannelCall("popup_settext", popup.ID, popup.Text)
		v.BatchChannelCall("popup_setoptions", popup.ID, map[string]interface{}{
			"time": 3000, // close after 3 seconds, as popup_notification()
		})
		v.MustBatchEnd()
	}

	v.rearrangeProgressPopups()
	return nil
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
	line := 1
	for i := range popups {
		if popups[i].LinePos != line {
			popups[i].LinePos = line
			v.BatchChannelCall("popup_setoptions",
				popups[i].ID,
				map[string]interface{}{"line": popups[i].LinePos},
			)
		}
		line += len(popups[i].Text) + 2 // lines + top & bottom border
	}
	v.MustBatchEnd()
}
