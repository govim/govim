// Command govim is a Vim8 channel-based plugin, written in Go, to support the writing of Go code in Vim8
package main

import (
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
	"gopkg.in/tomb.v2"
)

var (
	fTail = flag.Bool("tail", false, "whether to also log output to stdout")
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
	flag.Parse()

	if sock := os.Getenv("GOVIMTEST_SOCKET"); sock != "" {
		ln, err := net.Listen("tcp", sock)
		if err != nil {
			return fmt.Errorf("failed to listen on %v: %v", sock, err)
		}
		for {
			conn, err := ln.Accept()
			if err != nil {
				return fmt.Errorf("failed to accept connection on %v: %v", sock, err)
			}

			go func() {
				if err := launch(conn, conn); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}()
		}
	} else {
		return launch(os.Stdin, os.Stdout)
	}
}

func launch(in io.ReadCloser, out io.WriteCloser) error {
	defer out.Close()

	d := newDriver()

	nowStr := time.Now().Format("20060102_1504_05.000000000")
	tf, err := ioutil.TempFile("", "govim_"+nowStr+"_*")
	if err != nil {
		return fmt.Errorf("failed to create log file")
	}
	defer tf.Close()

	var log io.Writer = tf
	if *fTail {
		log = io.MultiWriter(tf, os.Stdout)
	}

	if os.Getenv("GOVIMTEST_SOCKET") != "" {
		fmt.Fprintf(os.Stderr, "New connection will log to %v\n", tf.Name())
	}

	g, err := govim.NewGoVim(in, out, log)
	if err != nil {
		return fmt.Errorf("failed to create govim instance: %v", err)
	}
	g.Init = d.init
	d.Govim = g

	d.Kill(g.Run())
	return d.Wait()
}

type driver struct {
	*plugin.Driver

	tomb tomb.Tomb
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

func (d *driver) init() error {
	d.ChannelEx(`augroup govim`)
	d.ChannelEx(`augroup END`)
	d.DefineFunction("Hello", []string{}, d.hello)
	d.DefineCommand("Hello", d.helloComm)

	return nil
}
