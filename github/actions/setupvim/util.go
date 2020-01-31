package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type command struct {
	cmd *exec.Cmd
}

type runCmdErr struct {
	error
}

func cmd(cmdname string, args ...string) *command {
	cmd := exec.Command(cmdname, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return &command{
		cmd: cmd,
	}
}

func (c *command) run() {
	format := "> "
	wd, err := os.Getwd()
	check(err, "failed to get working directory: %v", err)
	runDir := wd
	if d := c.cmd.Dir; d != "" {
		rd, err := filepath.Abs(d)
		check(err, "failed to make %v absolute: %v", d, err)
		runDir = rd
	}
	if wd != runDir {
		format += fmt.Sprintf("cd %v; ", runDir)
	}
	format += "%v\n"
	fmt.Printf(format, strings.Join(c.cmd.Args, " "))
	if err := c.cmd.Run(); err != nil {
		panic(runCmdErr{err})
	}
}

func (c *command) runInDir(dir string) {
	c.cmd.Dir = dir
	c.run()
}

func tempDir(dir, pattern string) string {
	td, err := ioutil.TempDir(dir, pattern)
	check(err, "failed to create temp dir: %v", err)
	return td
}

func check(err error, format string, args ...interface{}) {
	if err != nil {
		panic(fmt.Errorf(format, args...))
	}
}
