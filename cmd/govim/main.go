// Command govim is a Vim8 channel-based plugin, written in Go, to support the writing of Go code in Vim8
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"

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

	nowStr := time.Now().Format("20060102_1504_05_999999999")
	tf, err := ioutil.TempFile("", nowStr+"_*")
	if err != nil {
		return fmt.Errorf("failed to create log file")
	}
	defer tf.Close()

	d := newDriver()
	g, err := govim.NewGoVim(in, out, tf)
	if err != nil {
		return fmt.Errorf("failed to create govim instance: %v", err)
	}

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
}

type parseData struct {
	fset *token.FileSet
	file *ast.File
}

func newDriver() *driver {
	return &driver{
		Driver: plugin.NewDriver("GOVIM"),
	}
}

func (d *driver) init(g *govim.Govim) error {
	d.Govim = g

	return d.Do(func() error {
		d.ChannelEx(`augroup govim`)
		d.ChannelEx(`augroup END`)
		d.DefineFunction("Hello", []string{}, d.hello)
		d.DefineCommand("Hello", d.helloComm)

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
