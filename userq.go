package govim

import (
	"encoding/json"
	"fmt"

	"gopkg.in/tomb.v2"
)

type userQInst struct {
	*govimImpl
}

var _ Govim = userQInst{}

func (u userQInst) ChannelRedraw(force bool) error {
	ch := make(scheduledCallback)
	err := u.govimImpl.channelRedrawImpl(ch, force)
	return u.handleUserQError(ch, err, channelRedrawErrMsg, force)
}

func (u userQInst) ChannelEx(expr string) error {
	ch := make(scheduledCallback)
	err := u.govimImpl.channelExImpl(ch, expr)
	return u.handleUserQError(ch, err, channelExErrMsg, expr)
}

func (u userQInst) ChannelNormal(expr string) error {
	ch := make(scheduledCallback)
	err := u.govimImpl.channelNormalImpl(ch, expr)
	return u.handleUserQError(ch, err, channelNormalErrMsg, expr)
}

func (u userQInst) ChannelExpr(expr string) (json.RawMessage, error) {
	ch := make(scheduledCallback)
	err := u.govimImpl.channelExprImpl(ch, expr)
	return u.handleUserQValueAndError(ch, err, channelExprErrMsg, expr)
}

func (u userQInst) ChannelCall(fn string, args ...interface{}) (json.RawMessage, error) {
	ch := make(scheduledCallback)
	err := u.govimImpl.channelCallImpl(ch, fn, args...)
	return u.handleUserQValueAndError(ch, err, channelCallErrMsg, fn, args)
}

func (u userQInst) Sync() Govim {
	return u
}

func (u userQInst) handleUserQError(ch scheduledCallback, err error, format string, args ...interface{}) error {
	_, err = u.handleUserQValueAndError(ch, err, format, args...)
	return err
}

func (u userQInst) handleUserQValueAndError(ch scheduledCallback, err error, format string, args ...interface{}) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	args = append([]interface{}{}, args...)
	select {
	case <-u.govimImpl.tomb.Dying():
		return nil, tomb.ErrDying
	case u.flushEvents <- struct{}{}:
		select {
		case <-u.govimImpl.tomb.Dying():
			return nil, tomb.ErrDying
		case resp := <-ch:
			if resp.errString != "" {
				args = append(args, resp.errString)
				return nil, fmt.Errorf(format, args...)
			}
			return resp.val, nil
		}
	}
}
