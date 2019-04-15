package govim

import (
	"encoding/json"
	"fmt"

	"github.com/kr/pretty"
)

const (
	sysFuncOnViewportChange = sysFuncPref + "OnViewportChange"
)

// SubOnViewportChange creates a subscription to the OnViewportChange event
// exposed by Govim
func (g *govimImpl) SubOnViewportChange(f func(Viewport) error) *OnViewportChangeSub {
	res := &OnViewportChangeSub{f: f}
	g.onViewportChangeSubsLock.Lock()
	g.onViewportChangeSubs = append(g.onViewportChangeSubs, res)
	g.onViewportChangeSubsLock.Unlock()
	return res
}

// UnsubOnViewportChange removes a subscription to the OnViewportChange event.
// It panics if sub is not an active subscription.
func (g *govimImpl) UnsubOnViewportChange(sub *OnViewportChangeSub) {
	g.onViewportChangeSubsLock.Lock()
	defer g.onViewportChangeSubsLock.Unlock()
	for i, s := range g.onViewportChangeSubs {
		if sub == s {
			g.onViewportChangeSubs = append(g.onViewportChangeSubs[:i], g.onViewportChangeSubs[i+1:]...)
			return
		}
	}
	panic(fmt.Errorf("did not find subscription"))
}

func (g *govimImpl) ToggleOnViewportChange() {
	select {
	case <-g.tomb.Dying():
		// we are already dying, nothing to do
	case resp := <-g.unscheduledCallCallback("toggleUpdateViewport"):
		if resp.errString != "" {
			g.errProto("failed to toggle OnViewportChange: %v", resp.errString)
		}
	}
}

type OnViewportChangeSub struct {
	f func(Viewport) error
}

func (g *govimImpl) onViewportChange(args ...json.RawMessage) (interface{}, error) {
	var r Viewport
	g.decodeJSON(args[0], &r)
	g.viewportLock.Lock()
	g.currViewport = r
	r = r.dup()
	g.viewportLock.Unlock()

	g.Logf("Viewport changed: %v", pretty.Sprint(r))

	var subs []*OnViewportChangeSub
	g.onViewportChangeSubsLock.Lock()
	subs = append(subs, g.onViewportChangeSubs...)
	g.onViewportChangeSubsLock.Unlock()
	for _, s := range subs {
		if err := s.f(r.dup()); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

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
func (g *govimImpl) Viewport() Viewport {
	var res Viewport
	g.viewportLock.Lock()
	res = g.currViewport.dup()
	g.viewportLock.Unlock()
	return res
}

func (v Viewport) dup() Viewport {
	v.Windows = append([]WinInfo{}, v.Windows...)
	return v
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
