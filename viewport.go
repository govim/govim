package govim

import (
	"encoding/json"
	"fmt"
)

type Viewport struct {
	Current WinInfo
	Windows []WinInfo
}

type WinInfo struct {
	WinNr    int
	BotLine  int
	Height   int
	BufNr    int
	WinBar   int
	Width    int
	TabNr    int
	QuickFix bool
	TopLine  int
	LocList  bool
	WinCol   int
	WinRow   int
	WinID    int
	Terminal bool
}

// Viewport returns the active Vim viewport
func (g *govimImpl) Viewport() (vp Viewport, err error) {
	// Calling s:buildCurrentViewport is something of a legacy from the previous
	// attempt to have Vim push viewport changes to the plugin. In that setup,
	// we called s:buildCurrentViewport on a timer and pushed changes to govim
	// if the viewport had changed. For easy of reverting the change to remove the
	// viewport code, we leave s:buildCurrentViewport in place, not least because
	// it's nice an efficient to do it as one-shot in VimScript.
	res, err := g.Scheduled().ChannelExpr("s:buildCurrentViewport()")
	if err != nil {
		err = fmt.Errorf("failed to build current viewport: %v", err)
		return
	}
	g.decodeJSON(res, &vp)
	return
}

func (wi *WinInfo) UnmarshalJSON(b []byte) error {
	var w struct {
		WinNr    int `json:"winnr"`
		BotLine  int `json:"botline"`
		Height   int `json:"height"`
		BufNr    int `json:"bufnr"`
		WinBar   int `json:"winbar"`
		Width    int `json:"width"`
		TabNr    int `json:"tabnr"`
		QuickFix int `json:"quickfix"`
		TopLine  int `json:"topline"`
		LocList  int `json:"loclist"`
		WinCol   int `json:"wincol"`
		WinRow   int `json:"winrow"`
		WinID    int `json:"winid"`
		Terminal int `json:"terminal"`
	}

	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}

	wi.WinNr = w.WinNr
	wi.BotLine = w.BotLine
	wi.Height = w.Height
	wi.BufNr = w.BufNr
	wi.WinBar = w.WinBar
	wi.Width = w.Width
	wi.TabNr = w.TabNr
	wi.QuickFix = w.QuickFix == 1
	wi.TopLine = w.TopLine
	wi.LocList = w.LocList == 1
	wi.WinCol = w.WinCol
	wi.WinRow = w.WinRow
	wi.WinID = w.WinID
	wi.Terminal = w.Terminal == 1

	return nil
}
