// +build darwin

package fswatcher

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsevents"
	"gopkg.in/tomb.v2"
)

const (
	fRemoved = fsevents.ItemRemoved | fsevents.ItemRenamed
	fChanged = fsevents.ItemModified | fsevents.ItemChangeOwner
	fCreated = fsevents.ItemCreated
	fIsDir   = fsevents.ItemIsDir
)

type fswatcher struct {
	eventCh chan Event
	es      *fsevents.EventStream

	// Darwin do recursive watching so we need to filter files in directories that
	// wasn't explicitly added. Note that it is desirable to watch recursively to avoid
	// a data race (#492).
	watched     map[string]bool // keyed by full path to directory
	watchedLock sync.RWMutex
}

func New(gomodpath string, tomb *tomb.Tomb) (*FSWatcher, error) {
	dirpath := filepath.Dir(gomodpath)
	dev, err := fsevents.DeviceForPath(dirpath)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve device for path %v: %v", dirpath, err)
	}

	es := &fsevents.EventStream{
		Paths:   []string{dirpath},
		Latency: 200 * time.Millisecond,
		Device:  dev,
		Flags:   fsevents.FileEvents | fsevents.WatchRoot,
	}

	es.Start()

	// fsevents returns paths relative to device root so we need
	// to figure out the actual mount point
	mountPoint, err := filepath.Abs(dirpath)
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
	w := &fswatcher{eventCh, es, map[string]bool{}, sync.RWMutex{}}

	tomb.Go(func() error {
		for {
			events, ok := <-es.Events
			if !ok {
				break
			}
			for i := range events {
				event := events[i]

				path := filepath.Join(mountPoint, event.Path)
				var dir string
				if !(event.Flags&fIsDir > 0) {
					dir = filepath.Dir(path)
				} else {
					dir = path
				}
				w.watchedLock.RLock()
				if !w.watched[dir] {
					w.watchedLock.RUnlock()
					continue
				}
				w.watchedLock.RUnlock()

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

	return &FSWatcher{w}, nil
}

func (w *fswatcher) Add(path string) error {
	w.watchedLock.Lock()
	w.watched[path] = true
	w.watchedLock.Unlock()
	return nil
}

func (w *fswatcher) Remove(path string) error {
	w.watchedLock.Lock()
	delete(w.watched, path)
	w.watchedLock.Unlock()
	return nil
}

func (w *fswatcher) Close() error {
	w.es.Stop()
	return nil
}
func (w *fswatcher) Events() chan Event { return w.eventCh }
func (w *fswatcher) Errors() chan error { return make(chan error) }
