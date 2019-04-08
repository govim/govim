package govim

import (
	"encoding/json"
	"fmt"
	"io"
)

type chanErrHandler func(chan callbackResp, error) error
type chanValueErrHandler func(chan callbackResp, error) (json.RawMessage, error)

func handleChannelError(format string, args ...interface{}) func(chan callbackResp, error) error {
	f := handleChannelValueAndError(format, args...)
	return func(ch chan callbackResp, err error) error {
		_, err = f(ch, err)
		return err
	}
}

func handleChannelValueAndError(format string, args ...interface{}) func(chan callbackResp, error) (json.RawMessage, error) {
	args = append([]interface{}{}, args...)
	return func(ch chan callbackResp, err error) (json.RawMessage, error) {
		resp := <-ch
		if resp.errString != "" {
			args = append(args, resp.errString)
			return nil, fmt.Errorf(format, args...)
		}
		return resp.val, nil
	}
}

// ChannelRedraw implements Govim.ChannelRedraw
func (g *govimImpl) ChannelRedraw(force bool) error {
	f := handleChannelError(channelRedrawErrMsg, force)
	return g.channelRedrawImpl(f, force)
}

const channelRedrawErrMsg = "failed to redraw (force = %v) in Vim: %v"

func (g *govimImpl) channelRedrawImpl(f chanErrHandler, force bool) error {
	<-g.loaded
	var sForce string
	if force {
		sForce = "force"
	}
	var err error
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback("redraw", sForce)
	})
	return f(ch, err)
}

// ChannelEx implements Govim.ChannelEx
func (g *govimImpl) ChannelEx(expr string) error {
	f := handleChannelError(channelExErrMsg, expr)
	return g.channelExImpl(f, expr)
}

const channelExErrMsg = "failed to ex(%v) in Vim: %v"

func (g *govimImpl) channelExImpl(f chanErrHandler, expr string) error {
	<-g.loaded
	var err error
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback("ex", expr)
	})
	return f(ch, err)
}

// ChannelEx implements Govim.ChannelNormal
func (g *govimImpl) ChannelNormal(expr string) error {
	f := handleChannelError(channelNormalErrMsg, expr)
	return g.channelNormalImpl(f, expr)
}

const channelNormalErrMsg = "failed to normal(%v) in Vim: %v"

func (g *govimImpl) channelNormalImpl(f chanErrHandler, expr string) error {
	<-g.loaded
	var err error
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback("normal", expr)
	})
	return f(ch, err)
}

// ChannelExpr implements Govim.ChannelExpr
func (g *govimImpl) ChannelExpr(expr string) (json.RawMessage, error) {
	f := handleChannelValueAndError(channelExprErrMsg, expr)
	return g.channelExprImpl(f, expr)
}

const channelExprErrMsg = "failed to expr(%v) in Vim: %v"

func (g *govimImpl) channelExprImpl(f chanValueErrHandler, expr string) (json.RawMessage, error) {
	<-g.loaded
	var err error
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback("expr", expr)
	})
	return f(ch, err)
}

// ChannelCall implements Govim.ChannelCall
func (g *govimImpl) ChannelCall(fn string, args ...interface{}) (json.RawMessage, error) {
	f := handleChannelValueAndError(channelCallErrMsg, fn, args)
	return g.channelCallImpl(f, fn, args...)
}

const channelCallErrMsg = "failed to call(%v) in Vim: %v"

func (g *govimImpl) channelCallImpl(f chanValueErrHandler, fn string, args ...interface{}) (json.RawMessage, error) {
	<-g.loaded
	args = append([]interface{}{fn}, args...)
	var err error
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback("call", args...)
	})
	return f(ch, err)
}

func (g *govimImpl) DoProto(f func()) (err error) {
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
				err = r
			default:
				panic(r)
			}
		}
	}()
	f()
	return
}
