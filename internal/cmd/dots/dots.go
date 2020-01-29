package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"time"
)

var (
	fTickInternal = flag.String("tick", "1m", "How often to output a 'tick'")
)

func main() {
	os.Exit(main1())
}

func main1() int {
	if err := mainerr(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func mainerr() error {
	flag.Parse()
	tickInterval, err := time.ParseDuration(*fTickInternal)
	if err != nil {
		return fmt.Errorf("failed to parse tick interval %q: %v", *fTickInternal, err)
	}
	args := flag.Args()
	if len(args) == 0 {
		return fmt.Errorf("need a command")
	}
	done := make(chan struct{})
	progressDone := make(chan struct{})
	var wrote bool
	go func() {
		tick := time.NewTicker(tickInterval)
	Progress:
		for {
			select {
			case <-tick.C:
				fmt.Printf(".")
				wrote = true
			case <-done:
				break Progress
			}
		}
		tick.Stop()
		close(progressDone)
	}()
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	close(done)
	<-progressDone
	if wrote {
		fmt.Println("")
	}
	if err != nil {
		return fmt.Errorf("failed to run %v: %v\n%s", strings.Join(cmd.Args, " "), err, out)
	}
	fmt.Printf("%s", out)
	return nil
}
