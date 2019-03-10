package main // import "myitcv.io/govim/examples/asthighlight"

import (
	"myitcv.io/govim"
)

type Asdf struct{}

var _ govim.Plugin

func (p *Asdf) Init() {}

var Plugin = Asdf{}
