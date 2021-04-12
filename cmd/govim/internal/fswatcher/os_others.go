//go:build !darwin
// +build !darwin

package fswatcher

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/tomb.v2"
)

type fswatcher struct {
	eventCh chan Event
	errCh   chan error
	mw      *fsnotify.Watcher
	logf    logFn

	// activeWatchers is a map keyed by directory paths for directories that has been
	// added to watch. The bool value is used to dedup multiple remove events for the same
	// directory from fsnotify (inotify sends two events when removing a dir, "DELETE_SELF"
	// and "DELETE,ISDIR" where the difference isn't exposed by fsnotify). The value is true
	// if the watcher is active, and false for directories that no longer are watched.
	// By only sending events on the transition from true to false we ensure that only one
	// event is sent even when we get duplicate remove events. The key must be removed if
	// a file is created with the same name as a previous directory.
	activeWatches map[string]bool
}

// populateWatches is used to walk the provided path and add/remove directories to the
// underlying fsnotify watcher since it isn't recursive. It return all files found in
// watched directories during the walk. If the initPath is a newly created directory
// we must also send create events for the returned files to prevent a race condition where
// files are created before the directory is watched.
func (w *fswatcher) populateWatches(initPath string, filter watchFilterFn) ([]string, error) {
	var files []string
	err := filepath.Walk(initPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// fast path for directories that are already watched
		if w.activeWatches[path] {
			return nil
		}

		if info.IsDir() && filter(path) {
			// We might end up here if the user creates a go.mod in a watched dir
			// for example. Then we need to remove the watch.
			if v, ok := w.activeWatches[path]; ok && v {
				w.logf("stopped watching dir %q", path)
				w.mw.Remove(path)
				w.activeWatches[path] = false
			}
			return filepath.SkipDir
		}

		if !info.IsDir() {
			files = append(files, path)
			return nil
		}

		if v, ok := w.activeWatches[path]; !ok || !v {
			if err := w.mw.Add(path); err != nil {
				return err
			}
			w.logf("started watching dir %q", path)
			w.activeWatches[path] = true
			// When mw.Add returns the folder might already contain files and/or
			// directories that we must send events for manually. The recommended
			// way in inotify(7) is to:
			//
			// "[...] new files (and subdirectories) may already exist inside the
			// subdirectory. Therefore, you might want to scan the contents of ths
			// subdirectory immediately after adding the watch (and, if desired,
			// recursively add watchers for any subdirectories that it contains).
			fs, err := w.populateWatches(path, filter)
			files = append(files, fs...)
			if err != nil {
				return err
			}
			// We must stop walking here since we initiated a new walk above, to avoid
			// duplicate events..
			return filepath.SkipDir
		}
		return nil
	})
	return files, err
}

// New creates a file watcher that provide events recursively for all files and directories
// that aren't filtered by the watch filter. The root argument must be an existing directory,
// in our case the module root. FSWatcher will not send events for any path (file or directory)
// where the filter returns "true".
func New(root string, filter watchFilterFn, logf logFn, tomb *tomb.Tomb) (*FSWatcher, error) {
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("provided root %q must be an existing directory", root)
	}

	mw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create new watcher: %v", err)
	}

	eventCh := make(chan Event)
	w := &fswatcher{
		eventCh:       eventCh,
		errCh:         mw.Errors,
		mw:            mw,
		logf:          logf,
		activeWatches: make(map[string]bool),
	}

	if _, err := w.populateWatches(root, filter); err != nil {
		return nil, fmt.Errorf("initial root walk failed: %w", err)
	}

	tomb.Go(func() error {
		for {
			e, ok := <-mw.Events
			if !ok {
				break
			}
			path := e.Name
			switch e.Op {
			// fsnotify processes file renaming as a Rename event followed by a
			// Create event, so we can effectively treat renaming as removal.
			case fsnotify.Rename, fsnotify.Remove:
				if v, ok := w.activeWatches[path]; ok {
					if v {
						logf("stopped watching (implicit) dir %q", path)
					}
					// We try to avoid sending remove events for directories, however
					// fsnotify send is two events per delete. One for the "DELETE_SELF",
					// and another for "DELETE,ISDIR". These flags aren't exposed by
					// fsnotify so after the file/directory has been removed there is
					// no way to tell if the path was a directory.
					// To prevent events for the second we just set the entry to false
					// w/o dropping it entirely.
					w.activeWatches[path] = false
					continue
				}
				eventCh <- Event{path, OpRemoved}
			case fsnotify.Chmod, fsnotify.Write, fsnotify.Create:
				var op Op
				if e.Op == fsnotify.Create {
					op = OpCreated
				} else {
					op = OpChanged
				}

				di, err := os.Stat(path)
				if err != nil {
					// This might happen for example when vim writes temporary files that
					// are removed before the stat call.
					logf("error: failed to stat %q: %v", path, err)
					continue
				}
				if !di.IsDir() {
					// We know for sure that this isn't a directory so we must remove
					// any entry from activeWatchers in case we have had a directory
					// with the same name earlier.
					delete(w.activeWatches, path)
					if !filter(path) {
						eventCh <- Event{path, op}
					}
					continue
				}
				files, err := w.populateWatches(path, filter)
				if err != nil {
					logf("error: failed to walk %q: %v", path, err)
					continue
				}
				// If the directory was just created, we need to send file creation events
				// for all files created as well.
				if op == OpCreated && w.activeWatches[path] {
					for _, f := range files {
						eventCh <- Event{f, op}
					}
				}
			}
		}
		close(eventCh)
		return nil
	})

	return &FSWatcher{w}, nil
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
