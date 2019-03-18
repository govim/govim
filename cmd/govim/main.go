// Command govim is a Vim8 channel-based plugin, written in Go, to support the writing of Go code in Vim8
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/internal/plugin"
)

func main() {
	os.Exit(main1())
}

func main1() int {
	switch err := mainerr(); err {
	case nil:
		return 0
	case flag.ErrHelp:
		return 2
	default:
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
}

func mainerr() error {
	var in io.ReadCloser
	var out io.WriteCloser

	if sock := os.Getenv("GOVIMTEST_SOCKET"); sock != "" {
		ln, err := net.Listen("tcp", sock)
		if err != nil {
			return fmt.Errorf("failed to listen on %v: %v", sock, err)
		}
		conn, err := ln.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept connection on %v: %v", sock, err)
		}
		in, out = conn, conn
	} else {
		in, out = os.Stdin, os.Stdout
	}
	d := newDriver()
	g, err := govim.NewGoVim(in, out)
	if err != nil {
		return fmt.Errorf("failed to create govim instance: %v", err)
	}

	// TODO set a logger/similar here for the govim plugin?

	runCh := make(chan error)

	go func() {
		if err := g.Run(); err != nil {
			runCh <- fmt.Errorf("error whilst running govim instance: %v", err)
		}
		close(runCh)
	}()

	if err := d.init(g); err != nil {
		return nil
	}

	if err, ok := <-runCh; ok && err != nil {
		return err
	}
	return nil
}

type driver struct {
	*plugin.Driver

	// TODO does this need some sort of locking?
	buffSyntax map[int]*synGenerator
}

func newDriver() *driver {
	return &driver{
		Driver:     plugin.NewDriver("GOVIM"),
		buffSyntax: make(map[int]*synGenerator),
	}
}

func (d *driver) init(g *govim.Govim) error {
	d.Govim = g
	return d.Do(func() error {
		d.DefineFunction("Hello", []string{}, d.hello)
		d.DefineCommand("Hello", d.helloComm)
		d.DefineAutoCommand("", govim.Events{govim.EventBufReadPost, govim.EventCursorMoved, govim.EventTextChanged, govim.EventTextChangedI}, govim.Patterns{"*.go"}, false, d.highlight)

		// is this the correct hack for the fact the plugin is loaded async?
		d.ChannelEx("doau BufReadPost *.go")
		return nil
	})
}

func (d *driver) hello(args ...json.RawMessage) (interface{}, error) {
	return "Hello from function", nil
}

func (d *driver) helloComm(flags govim.CommandFlags, args ...string) error {
	d.ChannelEx(`echom "Hello from command"`)
	return nil
}
