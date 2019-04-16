package govim

import (
	"encoding/json"
	"fmt"

	"gopkg.in/tomb.v2"
)

type eventQueueInst struct {
	*govimImpl
}

var _ Govim = eventQueueInst{}

func (e eventQueueInst) ChannelRedraw(force bool) error {
	ch := make(scheduledCallback)
	err := e.govimImpl.channelRedrawImpl(ch, force)
	return e.handleUserQError(ch, err, channelRedrawErrMsg, force)
}

func (e eventQueueInst) ChannelEx(expr string) error {
	ch := make(scheduledCallback)
	err := e.govimImpl.channelExImpl(ch, expr)
	return e.handleUserQError(ch, err, channelExErrMsg, expr)
}

func (e eventQueueInst) ChannelNormal(expr string) error {
	ch := make(scheduledCallback)
	err := e.govimImpl.channelNormalImpl(ch, expr)
	return e.handleUserQError(ch, err, channelNormalErrMsg, expr)
}

func (e eventQueueInst) ChannelExpr(expr string) (json.RawMessage, error) {
	ch := make(scheduledCallback)
	err := e.govimImpl.channelExprImpl(ch, expr)
	return e.handleUserQValueAndError(ch, err, channelExprErrMsg, expr)
}

func (e eventQueueInst) ChannelCall(fn string, args ...interface{}) (json.RawMessage, error) {
	ch := make(scheduledCallback)
	err := e.govimImpl.channelCallImpl(ch, fn, args...)
	return e.handleUserQValueAndError(ch, err, channelCallErrMsg, fn, args)
}

func (e eventQueueInst) Scheduled() Govim {
	return e
}

func (e eventQueueInst) Schedule(f func(Govim) error) chan struct{} {
	panic(fmt.Errorf("attempt to schedule work on the event queue from the event queue itself"))
}

func (e eventQueueInst) handleUserQError(ch scheduledCallback, err error, format string, args ...interface{}) error {
	_, err = e.handleUserQValueAndError(ch, err, format, args...)
	return err
}

func (e eventQueueInst) handleUserQValueAndError(ch scheduledCallback, err error, format string, args ...interface{}) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	args = append([]interface{}{}, args...)
	select {
	case <-e.govimImpl.tomb.Dying():
		return nil, tomb.ErrDying
	case e.flushEvents <- struct{}{}:
		select {
		case <-e.govimImpl.tomb.Dying():
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
