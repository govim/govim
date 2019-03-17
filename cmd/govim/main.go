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
	d := new(driver)
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
	*govim.Govim
}

func (d *driver) init(g *govim.Govim) error {
	d.Govim = g
	return d.do(func() error {
		d.DefineFunction("Hello", []string{}, d.hello)
		d.DefineCommand("HelloComm", d.helloComm)
		return nil
	})
}

func (d *driver) hello(args ...json.RawMessage) (interface{}, error) {
	return "World", nil
}

func (d *driver) helloComm(flags govim.CommandFlags, args ...string) error {
	d.ChannelEx(`echom "Hello world"`)
	return nil
}

func (d *driver) do(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r := r.(type) {
			case errDriver:
				err = r
			default:
				panic(r)
			}
		}
	}()
	return f()
}

func (d *driver) doFunction(f govim.VimFunction) govim.VimFunction {
	return func(args ...json.RawMessage) (interface{}, error) {
		var i interface{}
		err := d.do(func() error {
			var err error
			i, err = f(args...)
			return err
		})
		if err != nil {
			return nil, err
		}
		return i, nil
	}
}

func (d *driver) doRangeFunction(f govim.VimRangeFunction) govim.VimRangeFunction {
	return func(first, last int, args ...json.RawMessage) (interface{}, error) {
		var i interface{}
		err := d.do(func() error {
			var err error
			i, err = f(first, last, args...)
			return err
		})
		if err != nil {
			return nil, err
		}
		return i, nil
	}
}

func (d *driver) errorf(format string, args ...interface{}) {
	panic(errDriver{underlying: fmt.Errorf(format, args...)})
}

func (d *driver) ChannelExpr(expr string) json.RawMessage {
	i, err := d.Govim.ChannelExpr(expr)
	if err != nil {
		d.errorf("ChannelExpr(%q) failed: %v", expr, err)
	}
	return i
}

func (d *driver) ChannelEx(expr string) {
	if err := d.Govim.ChannelEx(expr); err != nil {
		d.errorf("ChannelEx(%q) failed: %v", expr, err)
	}
}

func (d *driver) parseString(j json.RawMessage) string {
	var v string
	if err := json.Unmarshal(j, &v); err != nil {
		d.errorf("failed to parse string from %q: %v", j, err)
	}
	return v
}

func (d *driver) parseInt(j json.RawMessage) int {
	var v int
	if err := json.Unmarshal(j, &v); err != nil {
		d.errorf("failed to parse int from %q: %v", j, err)
	}
	return v
}

func (d *driver) ChannelExprf(format string, args ...interface{}) json.RawMessage {
	return d.ChannelExpr(fmt.Sprintf(format, args...))
}

func (d *driver) ChannelExf(format string, args ...interface{}) {
	d.ChannelEx(fmt.Sprintf(format, args...))
}

func (d *driver) DefineFunction(name string, args []string, f govim.VimFunction) {
	if err := d.Govim.DefineFunction(name, args, d.doFunction(f)); err != nil {
		d.errorf("failed to DefineFunction %q: %v", name, err)
	}
}

func (d *driver) DefineRangeFunction(name string, args []string, f govim.VimRangeFunction) {
	if err := d.Govim.DefineRangeFunction(name, args, d.doRangeFunction(f)); err != nil {
		d.errorf("failed to DefineRangeFunction %q: %v", name, err)
	}
}

type errDriver struct {
	underlying error
}

func (e errDriver) Error() string {
	return fmt.Sprintf("driver error: %v", e.underlying)
}
