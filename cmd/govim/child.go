package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
)

// commandName defines the top-level command names in child-parent mode
type commandName string

const (
	cmdNameGopls commandName = "gopls"
)

// goplsMethodName defines the method names for the gopls command
type goplsMethodName string

const (
	methodGoplsSymbol goplsMethodName = "Symbol"
)

// knownChildErr is a type used by a "child" instance of govim to bail out
// of processing in such a way that the panic-ed error is then returned
// to the caller of runAsChild
type knownChildErr error

func runAsChild() (err error) {
	defer func() {
		switch r := recover().(type) {
		case nil:
		case knownChildErr:
			err = r
		default:
			panic(r)
		}
	}()

	// errorf is a convenience function for throwning knownChildErr's
	errorf := func(format string, args ...interface{}) {
		panic(knownChildErr(fmt.Errorf(format, args...)))
	}

	// TODO: support TCP?
	conn, err := net.Dial("unix", *fParent)
	if err != nil {
		errorf("failed to dial parent at %v: %v", *fParent, err)
	}

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	if err := enc.Encode(flagSet.Args()); err != nil {
		return fmt.Errorf("failed to encode args: %v", err)
	}

	// decode is a convenience decoder that handles receving
	// unexpected io.EOF errors
	decode := func(thing string, i interface{}) {
		err := dec.Decode(i)
		if err == nil {
			return
		}
		if err == io.EOF {
			// By definition we have not seen an exit code yet
			// because if we had we would have returned
			errorf("connection closed before exit code received")
		}
		errorf("failed to decode %v: %v", thing, err)
	}

	// now continually decode until we get an io.EOF
	for {
		var dest encodeCode
		decode("dest", &dest)
		var val interface{}
		switch dest {
		case encodeCodeJSONStdout, encodeCodeJSONStderr:
			var raw json.RawMessage
			decode("value", &raw)
			val = raw
		case encodeCodeRawStdout, encodeCodeRawStderr:
			var v string
			decode("value", &v)
			val = v
		case encodeCodeExitCode:
			var exitCode int
			decode("exitCode", &exitCode)
			return exitErr(exitCode)
		default:
			errorf("unknown destination for decoding: %v", dest)
		}
		switch dest {
		case encodeCodeRawStdout, encodeCodeJSONStdout:
			fmt.Fprintf(os.Stdout, "%s", val)
		case encodeCodeRawStderr, encodeCodeJSONStderr:
			fmt.Fprintf(os.Stderr, "%s", val)
		default:
			errorf("unknown destination for output: %v", dest)
		}
	}
}
