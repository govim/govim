// +build darwin dragonfly freebsd js,wasm linux nacl netbsd openbsd solaris windows

package main

import (
	"os"
	"syscall"
)

// ExitCode returns the exit code of the exited process, or -1
// if the process hasn't exited or was terminated by a signal.
func ExitCode(p *os.ProcessState) int {
	// return -1 if the process hasn't started.
	if p == nil {
		return -1
	}
	return p.Sys().(syscall.WaitStatus).ExitStatus()
}
