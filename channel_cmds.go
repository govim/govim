package govim

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/myitcv/govim/internal/transport"
	"gopkg.in/tomb.v2"
)

func (g *govimImpl) handleChannelError(ch transport.UnscheduledCallback, err error, format string, args ...interface{}) error {
	_, err = g.handleChannelValueAndError(ch, err, format, args)
	return err
}

func (g *govimImpl) handleChannelValueAndError(ch transport.UnscheduledCallback, err error, format string, args ...interface{}) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	args = append([]interface{}{}, args...)
	select {
	case <-g.tomb.Dying():
		panic(ErrShuttingDown)
	case resp := <-ch:
		if resp.ErrString != "" {
			args = append(args, resp.ErrString)
			return nil, fmt.Errorf(format, args...)
		}
		return resp.Val, nil
	}
}

// ChannelRedraw implements Govim.ChannelRedraw
func (g *govimImpl) ChannelRedraw(force bool) error {
	ch := make(transport.UnscheduledCallback)
	err := g.channelRedrawImpl(ch, force)
	return g.handleChannelError(ch, err, channelRedrawErrMsg, force)
}

const channelRedrawErrMsg = "failed to redraw (force = %v) in Vim: %v"

func (g *govimImpl) channelRedrawImpl(ch transport.Callback, force bool) error {
	<-g.loaded
	var sForce string
	if force {
		sForce = "force"
	}
	return g.DoProto(func() error {
		return g.callVim(ch, "redraw", sForce)
	})
}

// ChannelEx implements Govim.ChannelEx
func (g *govimImpl) ChannelEx(expr string) error {
	ch := make(transport.UnscheduledCallback)
	err := g.channelExImpl(ch, expr)
	return g.handleChannelError(ch, err, channelExErrMsg, expr)
}

const channelExErrMsg = "failed to ex(%v) in Vim: %v"

func (g *govimImpl) channelExImpl(ch transport.Callback, expr string) error {
	<-g.loaded
	return g.DoProto(func() error {
		return g.callVim(ch, "ex", expr)
	})
}

// ChannelNormal implements Govim.ChannelNormal
func (g *govimImpl) ChannelNormal(expr string) error {
	ch := make(transport.UnscheduledCallback)
	err := g.channelNormalImpl(ch, expr)
	return g.handleChannelError(ch, err, channelNormalErrMsg, expr)
}

const channelNormalErrMsg = "failed to normal(%v) in Vim: %v"

func (g *govimImpl) channelNormalImpl(ch transport.Callback, expr string) error {
	<-g.loaded
	return g.DoProto(func() error {
		return g.callVim(ch, "normal", expr)
	})
}

// ChannelExpr implements Govim.ChannelExpr
func (g *govimImpl) ChannelExpr(expr string) (json.RawMessage, error) {
	ch := make(transport.UnscheduledCallback)
	err := g.channelExprImpl(ch, expr)
	return g.handleChannelValueAndError(ch, err, channelExprErrMsg, expr)
}

const channelExprErrMsg = "failed to expr(%v) in Vim: %v"

func (g *govimImpl) channelExprImpl(ch transport.Callback, expr string) error {
	<-g.loaded
	return g.DoProto(func() error {
		return g.callVim(ch, "expr", expr)
	})
}

// ChannelCall implements Govim.ChannelCall
func (g *govimImpl) ChannelCall(fn string, args ...interface{}) (json.RawMessage, error) {
	ch := make(transport.UnscheduledCallback)
	err := g.channelCallImpl(ch, fn, args...)
	return g.handleChannelValueAndError(ch, err, channelCallErrMsg, fn, args)
}

const channelCallErrMsg = "failed to call(%v) in Vim: %v"

func (g *govimImpl) channelCallImpl(ch transport.Callback, fn string, args ...interface{}) error {
	<-g.loaded
	args = append([]interface{}{fn}, args...)
	return g.DoProto(func() error {
		return g.callVim(ch, "call", args...)
	})
}

func (g *govimImpl) DoProto(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r := r.(type) {
			case errProto:
				if r.underlying == io.EOF {
					g.logVimEventf("closing connection\n")
					return
				}
				err = r
			case error:
				if r == tomb.ErrDying {
					panic(ErrShuttingDown)
				}
				if r == ErrShuttingDown {
					panic(r)
				}
				err = r
			default:
				panic(r)
			}
		}
	}()
	err = f()
	return
}
