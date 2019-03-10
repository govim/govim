package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(main1())
}

func main1() int {
	switch err := mainerr(os.Stdin, os.Stdout); err {
	case nil:
		return 0
	case flag.ErrHelp:
		return 2
	default:
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
}

func mainerr(in io.Reader, out io.Writer) (retErr error) {
	log, err := os.OpenFile("/tmp/govim.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}
	logf := func(format string, args ...interface{}) {
		fmt.Fprintf(log, format, args...)
	}
	go func() {
		fmt.Printf(`["normal","iThis is a test"]`)
		fmt.Printf(`["ex", "w! blah.txt"]`)
		fmt.Printf(`["redraw", ""]`)
	}()
	dec := json.NewDecoder(os.Stdin)
	for {
		logf("waiting for message\n")
		var msg [2]json.RawMessage
		if err := dec.Decode(&msg); err != nil {
			panic(err)
		}
		var i int
		if err := json.Unmarshal(msg[0], &i); err != nil {
			panic(err)
		}

		var intf interface{}
		if err := json.Unmarshal(msg[1], &intf); err != nil {
			panic(err)
		}

		logf("%v: %v (%T)\n", i, intf, intf)
	}

	return nil
}

type govim struct {
}

type stateFn func(*govim, json.RawMessage) stateFn
