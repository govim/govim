package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/fswatcher"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

type modWatcher struct {
	// We don't use the *vimstate type because we are operating outside of the Vim/vimstate
	// "thread". Perhaps slightly inefficient that we query Vim to see whether a buffer is
	// loaded or not, but that should be de minimums... and in any case this hack will soon
	// disappear
	*govimplugin

	watcher *fswatcher.FSWatcher

	// root is the directory root of the watch
	root string

	// watches is the set of current watches "open" in the watcher
	watches map[string]bool
}

func (m *modWatcher) close() error { return m.watcher.Close() }

// newWatcher returns a new watcher that will "watch" on the Go files in the
// module identified by gomodpath
func newModWatcher(plug *govimplugin, gomodpath string) (*modWatcher, error) {
	w, err := fswatcher.New(gomodpath, &plug.tomb)
	if err != nil {
		return nil, err
	}

	dirpath := filepath.Dir(gomodpath)
	dir, err := os.Stat(dirpath)
	if err != nil || !dir.IsDir() {
		return nil, fmt.Errorf("could not resolve dir from go.mod path %v: %v", gomodpath, err)
	}

	res := &modWatcher{
		govimplugin: plug,
		watcher:     w,
		root:        dirpath,
		watches:     make(map[string]bool),
	}

	go res.watch()
	// fake event to kick start the watching
	res.watcher.Events() <- fswatcher.Event{
		Path: dirpath,
		Op:   fswatcher.OpChanged,
	}

	return res, nil
}

func (m *modWatcher) watch() {
	errf := func(format string, args ...interface{}) {
		m.Logf("**** file watcher error: "+format, args...)
	}
	infof := func(format string, args ...interface{}) {
		m.Logf("file watcher event: "+format, args...)
	}
	eventCh := m.watcher.Events()
	errCh := m.watcher.Errors()

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				// watcher has been stopped?
				return
			}
			switch event.Op {
			case fswatcher.OpRemoved:
				path := event.Path
				var didFind bool
				for ew := range m.watches {
					if event.Path == ew || strings.HasPrefix(ew, event.Path+string(os.PathSeparator)) {
						didFind = true
						if err := m.watcher.Remove(ew); err != nil {
							errf("failed to remove watch on %v: %v", ew, err)
						}
						infof("removed watch on %v", ew)
					}
				}
				if didFind {
					// it was a directory
					continue
				}
				if !ofInterest(path) {
					continue
				}
				m.Enqueue(func(govim.Govim) error {
					return m.vimstate.handleEvent(event)
				})
			case fswatcher.OpChanged, fswatcher.OpCreated:
				path := event.Path
				dirInfo, err := os.Stat(path)
				if err != nil {
					errf("failed to stat %v: %v", path, err)
					continue
				}
				if !dirInfo.IsDir() {
					// Is Vim handling this file? Is this a file we care about?
					if !ofInterest(path) {
						continue
					}
					m.Enqueue(func(govim.Govim) error {
						return m.vimstate.handleEvent(event)
					})
					continue
				}

				// Walk the dir that is event.Name. Because fsnotify isn't recursive,
				// we must manually install watches ourselves.
				// Note that this has a race condition:
				// https://github.com/govim/govim/issues/492
				err = filepath.Walk(event.Path, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() {
						return nil
					}

					// We have a dir
					switch filepath.Base(path)[0] {
					case '.', '_':
						return filepath.SkipDir
					}
					if path != m.root {
						// check we are not in a submodule
						if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
							return filepath.SkipDir
						}
					}
					err = m.watcher.Add(path)
					if err != nil {
						m.watches[path] = true
						infof("added watch on %v", path)
					}
					return err
				})
				if err != nil {
					errf("failed to walk %v: %v", event.Path, err)
				}
			}
		case err, ok := <-errCh:
			if !ok {
				// watcher has been stopped?
				return
			}
			// TODO: handle this case better
			m.Logf("***** file watcher error: %v", err)
		}
	}
}

func ofInterest(path string) bool {
	return filepath.Ext(path) == ".go" || filepath.Base(path) == "go.mod" || filepath.Base(path) == "go.sum"
}

func (v *vimstate) handleEvent(event fswatcher.Event) error {
	// We are handling a filesystem event... so the best we can do is log errors
	errf := func(format string, args ...interface{}) {
		v.Logf("**** handleEvent error: "+format, args...)
	}

	var changeType protocol.FileChangeType
	switch event.Op {
	case fswatcher.OpRemoved:
		changeType = protocol.Deleted
	case fswatcher.OpCreated:
		changeType = protocol.Created
	case fswatcher.OpChanged:
		changeType = protocol.Changed
	default:
		panic(fmt.Errorf("unknown fswatcher event type: %v", event))
	}

	uri := span.URIFromPath(event.Path)
	v.autoreadBuffer(uri)

	params := &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{
			{URI: protocol.DocumentURI(uri), Type: changeType},
		},
	}
	err := v.server.DidChangeWatchedFiles(context.Background(), params)
	if err != nil {
		errf("failed to call server.DidChangeWatchedFiles: %v", err)
	}
	v.Logf("handleEvent: handled %v", event)
	return nil
}

func (v *vimstate) autoreadBuffer(uri span.URI) {
	if v.config.ExperimentalAutoreadLoadedBuffers == nil || !*v.config.ExperimentalAutoreadLoadedBuffers {
		return
	}

	for _, b := range v.buffers {
		if b.URI().Filename() == uri.Filename() {
			v.ChannelEx(fmt.Sprintf("checktime %d", b.Num))
		}
	}
}
