//go:build darwin
// +build darwin

package fswatcher

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsevents"
	"gopkg.in/tomb.v2"
)

const (
	fRemoved = fsevents.ItemRemoved | fsevents.ItemRenamed
	fChanged = fsevents.ItemModified | fsevents.ItemChangeOwner
	fCreated = fsevents.ItemCreated
)

type fswatcher struct {
	eventCh chan Event
	es      *fsevents.EventStream
}

// New creates a file watcher that provide events recursively for all files and directories
// that aren't filtered by the watch filter. The root argument must be an existing directory,
// in our case the module root. FSWatcher will not send events for any path (file or directory)
// where the filter returns "true".
func New(root string, filter watchFilterFn, logf logFn, tomb *tomb.Tomb) (*FSWatcher, error) {
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("provided root %q must be an existing directory", root)
	}
	dev, err := fsevents.DeviceForPath(root)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve device for path %v: %v", root, err)
	}

	es := &fsevents.EventStream{
		Paths:   []string{root},
		Latency: 200 * time.Millisecond,
		Device:  dev,
		Flags:   fsevents.FileEvents | fsevents.WatchRoot,
	}

	es.Start()

	// fsevents returns paths relative to device root so we need
	// to figure out the actual mount point
	mountPoint, err := filepath.Abs(root)
	if err != nil {
		log.Fatal(err)
	}

	for mountPoint != string(os.PathSeparator) {
		parent := filepath.Dir(mountPoint)
		pDev, err := fsevents.DeviceForPath(parent)
		if err != nil {
			log.Fatal(err)
		}
		if pDev != dev {
			break
		}
		mountPoint = parent
	}

	eventCh := make(chan Event)
	tomb.Go(func() error {
		for {
			events, ok := <-es.Events
			if !ok {
				break
			}
			for i := range events {
				event := events[i]
				path := filepath.Join(mountPoint, event.Path)

				if filter(path) {
					continue
				}

				// Darwin might include both "created" and "changed" in the same event
				// so ordering matters below. The "created" case should be checked
				// before "changed" to get a behavior that is more consistent with other
				// os_other.go.
				switch {
				case event.Flags&fRemoved > 0:
					eventCh <- Event{path, OpRemoved}
				case event.Flags&fCreated > 0:
					eventCh <- Event{path, OpCreated}
				case event.Flags&fChanged > 0:
					eventCh <- Event{path, OpChanged}
				}
			}
		}
		close(eventCh)
		return nil
	})

	return &FSWatcher{&fswatcher{eventCh, es}}, nil
}

func (w *fswatcher) Close() error {
	w.es.Stop()
	return nil
}
func (w *fswatcher) Events() chan Event { return w.eventCh }
func (w *fswatcher) Errors() chan error { return make(chan error) }
