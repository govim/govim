package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	flagSet = flag.NewFlagSet("govim", flag.ContinueOnError)
	fTail   = flagSet.Bool("tail", false, "whether to also log output to stdout")
	fParent = flagSet.String("parent", "", "the Unix Domain Socket on which a parent instance can be contacted")
)

func init() { flagSet.Usage = usage }

func usage() {
	fmt.Fprintf(os.Stderr, `
Usage of govim:

	govim [-tail] [-parent /path/to/uds] gopls ...

`[1:])
	flagSet.PrintDefaults()
}

type exitErr int

func (e exitErr) Error() string {
	return fmt.Sprintf("exit code: %v", int(e))
}

type usageErr string

func (u usageErr) Error() string { return string(u) }

type flagErr string

func (f flagErr) Error() string { return string(f) }

func main() { os.Exit(main1()) }

func main1() int {
	err := mainerr()
	if err == nil {
		return 0
	}
	switch err := err.(type) {
	case usageErr:
		fmt.Fprintln(os.Stderr, err)
		flagSet.Usage()
		return 2
	case exitErr:
		return int(err)
	case flagErr:
		return 2
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}
