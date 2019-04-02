package govim

import (
	"encoding/json"
	"fmt"
)

type userQInst struct {
	*govimImpl
}

var _ Govim = userQInst{}

func (u userQInst) ChannelRedraw(force bool) error {
	f := u.handleUserQError(channelRedrawErrMsg, force)
	return u.govimImpl.channelRedrawImpl(f, force)
}

func (u userQInst) ChannelEx(expr string) error {
	f := u.handleUserQError(channelExErrMsg, expr)
	return u.govimImpl.channelExImpl(f, expr)
}

func (u userQInst) ChannelNormal(normalpr string) error {
	f := u.handleUserQError(channelNormalErrMsg, normalpr)
	return u.govimImpl.channelNormalImpl(f, normalpr)
}

func (u userQInst) ChannelExpr(exprpr string) (json.RawMessage, error) {
	f := u.handleUserQValueAndError(channelExprErrMsg, exprpr)
	return u.govimImpl.channelExprImpl(f, exprpr)
}

func (u userQInst) ChannelCall(fn string, args ...interface{}) (json.RawMessage, error) {
	f := u.handleUserQValueAndError(channelCallErrMsg, fn, args)
	return u.govimImpl.channelCallImpl(f, fn, args...)
}

func (u userQInst) handleUserQError(format string, args ...interface{}) func(chan callbackResp, error) error {
	f := u.handleUserQValueAndError(format, args...)
	return func(ch chan callbackResp, err error) error {
		_, err = f(ch, err)
		return err
	}
}

func (u userQInst) Sync() Govim {
	return u
}

func (u userQInst) handleUserQValueAndError(format string, args ...interface{}) func(chan callbackResp, error) (json.RawMessage, error) {
	args = append([]interface{}{}, args...)
	return func(ch chan callbackResp, err error) (json.RawMessage, error) {
		if err != nil {
			return nil, err
		}
		var resp callbackResp
	WaitForResp:
		for {
			select {
			case resp = <-ch:
				break WaitForResp
			case u.flushEvents <- struct{}{}:
				// We have handed over to the eventQ
				// We need to unblock before we can proceed
				<-u.flushEvents
			}
		}
		if resp.errString != "" {
			args = append(args, resp.errString)
			return nil, fmt.Errorf(format, args...)
		}
		return resp.val, nil
	}
}
