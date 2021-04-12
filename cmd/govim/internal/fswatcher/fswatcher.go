// Package fswatcher is responsible for providing file system events to govim
package fswatcher

import "fmt"

type FSWatcher struct {
	*fswatcher // os specific
}

type Event struct {
	Path string
	Op   Op
}

func (e Event) String() string {
	return fmt.Sprintf("%s %q", e.Op, e.Path)
}

type Op string

const (
	OpChanged Op = "changed"
	OpRemoved Op = "removed"
	OpCreated Op = "created"
)

// watchFilterFn is used to determine if events should be sent for a particular
// file or directory. It enables language specific rules in a generic context.
type watchFilterFn func(path string) bool

type logFn func(format string, args ...interface{})
