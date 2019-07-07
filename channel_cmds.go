package govim

import (
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/tomb.v2"
)

// ChannelRedraw implements Govim.ChannelRedraw
func (g *govimImpl) ChannelRedraw(force bool) error {
	<-g.loaded
	var sForce string
	if force {
		sForce = "force"
	}
	if _, err := g.transport.SendAndReceive("redraw", sForce); err != nil {
		return fmt.Errorf(channelRedrawErrMsg, sForce, err)
	}
	return nil
}

const channelRedrawErrMsg = "failed to redraw (force = %v) in Vim: %v"

// ChannelEx implements Govim.ChannelEx
func (g *govimImpl) ChannelEx(expr string) error {
	<-g.loaded
	if _, err := g.transport.SendAndReceive("ex", expr); err != nil {
		return fmt.Errorf(channelExErrMsg, expr, err)
	}
	return nil
}

const channelExErrMsg = "failed to ex(%v) in Vim: %v"

// ChannelNormal implements Govim.ChannelNormal
func (g *govimImpl) ChannelNormal(expr string) error {
	<-g.loaded
	if _, err := g.transport.SendAndReceive("normal", expr); err != nil {
		return fmt.Errorf(channelNormalErrMsg, expr, err)
	}
	return nil
}

const channelNormalErrMsg = "failed to normal(%v) in Vim: %v"

// ChannelExpr implements Govim.ChannelExpr
func (g *govimImpl) ChannelExpr(expr string) (json.RawMessage, error) {
	<-g.loaded
	val, err := g.transport.SendAndReceive("expr", expr)
	if err != nil {
		return nil, fmt.Errorf(channelExprErrMsg, expr, err)
	}
	return val, nil
}

const channelExprErrMsg = "failed to expr(%v) in Vim: %v"

// ChannelCall implements Govim.ChannelCall
func (g *govimImpl) ChannelCall(fn string, args ...interface{}) (json.RawMessage, error) {
	<-g.loaded
	args = append([]interface{}{fn}, args...)
	val, err := g.transport.SendAndReceive("call", args...)
	if err != nil {
		return nil, fmt.Errorf(channelCallErrMsg, fn, args, err)
	}
	return val, nil
}

const channelCallErrMsg = "failed to call(%v(%v)) in Vim: %v"

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
