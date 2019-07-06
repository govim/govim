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

	Read() (int, json.RawMessage, error)
	SendJSON(p1, p2 interface{}, ps ...interface{})
}

type Callback struct{}
