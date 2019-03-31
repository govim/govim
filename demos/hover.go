package main

import (
	"flag"
	"fmt"
)

var (
	fFile = flag.String("file", "", "the file to open")
)

func main() {
	flag.Parse()

	fmt.Printf("We got -file flag value: %v\n")
}
