package transport

import (
	"encoding/json"
	"errors"
)

var (
	ErrShuttingDown = errors.New("govim shutting down")
)

type errProto struct {
	underlying error
}

type Transport interface {
	Start() error
	Close() error
	Loaded() chan struct{}
	Initialized() chan struct{}
	IsShutdown() chan struct{}

	Read() (id int, messageType string, args []json.RawMessage, err error)
	Send(callback Callback, callbackType string, params ...interface{}) error
	SendJSON(p1, p2 interface{}, ps ...interface{})
}

type Callback interface {
	isCallback()
}

// scheduledCallback is used for responses to calls to Vim made from the event queue
type ScheduledCallback chan CallbackResp

func (s ScheduledCallback) isCallback() {}

// unscheduledCallback is used for responses to calls made from off the event queue,
// i.e. as a result of a reponse from a process external to the plugin like gopls
type UnscheduledCallback chan CallbackResp

func (u UnscheduledCallback) isCallback() {}

// callbackResp is the container for a response from a call to callVim. If the
// call does not result in a value, e.g. ChannelEx, then val will be nil
type CallbackResp struct {
	ErrString string
	Val       json.RawMessage
}
