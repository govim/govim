package main

import (
	"encoding/json"

	"github.com/myitcv/govim"
)

func (d *driver) hello(args ...json.RawMessage) (interface{}, error) {
	return "Hello from function", nil
}

func (d *driver) helloComm(flags govim.CommandFlags, args ...string) error {
	d.ChannelEx(`echom "Hello from command"`)
	return nil
}
