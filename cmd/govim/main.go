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

	if err := govimInit(g); err != nil {
		return nil
	}

	if err, ok := <-runCh; ok && err != nil {
		return err
	}
	return nil
}

func govimInit(g *govim.Govim) error {
	if err := g.DefineFunction("Hello", []string{}, hello); err != nil {
		return fmt.Errorf("failed to DefineFunction: %v", err)
	}
	return nil
}

func hello(args ...json.RawMessage) (interface{}, error) {
	return "World", nil
}
