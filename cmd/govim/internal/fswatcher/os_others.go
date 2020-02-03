// +build !darwin

package fswatcher

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/tomb.v2"
)

type fswatcher struct {
	eventCh chan Event
	errCh   chan error
	mw      *fsnotify.Watcher
}

func New(gomodpath string, tomb *tomb.Tomb) (*FSWatcher, error) {
	mw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create new watcher: %v", err)
	}

	eventCh := make(chan Event)
	tomb.Go(func() error {
		for {
			e, ok := <-mw.Events
			if !ok {
				break
			}
			switch e.Op {
			case fsnotify.Rename, fsnotify.Remove:
				// fsnotify processes file renaming as a Rename event followed by a
				// Create event, so we can effectively treat renaming as removal.
				eventCh <- Event{e.Name, OpRemoved}
			case fsnotify.Chmod, fsnotify.Write:
				eventCh <- Event{e.Name, OpChanged}
			case fsnotify.Create:
				eventCh <- Event{e.Name, OpCreated}
			}
		}
		close(eventCh)
		return nil
	})

	return &FSWatcher{&fswatcher{
		eventCh: eventCh,
		errCh:   mw.Errors,
		mw:      mw,
	}}, nil
}

func (w *fswatcher) Add(path string) error {
	return w.mw.Add(path)
}

func (w *fswatcher) Remove(path string) error {
	return w.mw.Remove(path)
}

func (w *fswatcher) Close() error {
	return w.mw.Close()
}

func (w *fswatcher) Events() chan Event {
	return w.eventCh
}

func (w *fswatcher) Errors() chan error {
	return w.errCh
}
