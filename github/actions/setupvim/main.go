// setupvim is a pure Go GitHub Action for install Vim on various platforms. It
// is a work-in-progress.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/sethvargo/go-githubactions"
)

const debug = true

func main() {
	os.Exit(main1())
}

func main1() int {
	if err := mainerr(); err != nil {
		switch err := err.(type) {
		case runCmdErr:
			ee := err.error.(*exec.ExitError)
			return ee.ProcessState.ExitCode()
		default:
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}

func mainerr() error {
	td := tempDir("", "")
	if !debug {
		defer os.RemoveAll(td)
	}
	version := "master"
	if v := githubactions.GetInput("version"); v != "" {
		version = v
	}
	cmd("git", "clone", "-q", "--depth=1", "--single-branch", "--branch", version, "https://github.com/vim/vim", td).run()
	id := tempDir("", "")
	if !debug {
		defer os.RemoveAll(id)
	}
	cmd("./configure", "--prefix="+id, "--with-features=huge", "--enable-fail-if-missing").runInDir(td)
	cmd("make", "-j", strconv.Itoa(runtime.NumCPU())).runInDir(td)
	cmd("make", "install").runInDir(td)
	bin := filepath.Join(id, "bin")
	githubactions.SetEnv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return nil
}
