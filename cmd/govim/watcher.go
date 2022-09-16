package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/govim/govim"
	"github.com/govim/govim/cmd/govim/internal/fswatcher"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/protocol"
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
}

func (m *modWatcher) close() error { return m.watcher.Close() }

// newWatcher returns a new watcher that will "watch" on the Go files in the
// module identified by gomodpath
func newModWatcher(plug *govimplugin, gomodpath string) (*modWatcher, error) {
	infof := func(format string, args ...interface{}) {
		plug.Logf("file watcher event: "+format, args...)
	}

	dirpath := filepath.Dir(gomodpath)
	dir, err := os.Stat(dirpath)
	if err != nil || !dir.IsDir() {
		return nil, fmt.Errorf("could not resolve dir from go.mod path %v: %v", gomodpath, err)
	}

	w, err := fswatcher.New(dirpath, eventFilter(dirpath), infof, &plug.tomb)
	if err != nil {
		return nil, err
	}

	res := &modWatcher{
		govimplugin: plug,
		watcher:     w,
		root:        dirpath,
	}

	go res.watch()
	return res, nil
}

func (m *modWatcher) watch() {
	eventCh := m.watcher.Events()
	errCh := m.watcher.Errors()

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				// watcher has been stopped?
				return
			}

			if !ofInterest(event.Path) {
				continue
			}

			m.Enqueue(func(govim.Govim) error {
				return m.vimstate.handleEvent(event)
			})

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

// eventFilter return a function that is used to filter events according to the Go specific
// rules described by https://golang.org/cmd/go/#hdr-Package_lists_and_patterns:
//
//	> Directory and file names that begin with "." or "_" are ignored by the go
//	  tool, as are directories named "testdata".
//
// and the module boundary described by https://golang.org/ref/mod#modules-overview:
//
//	> The module root directory is the directory that contains the go.mod file.
func eventFilter(root string) func(string) bool {
	return func(path string) bool {
		path = filepath.Clean(path)
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			var filename string
			path, filename = filepath.Split(path)
			switch filename[0] {
			case '.', '_':
				return true
			}
		}

		// TODO: cache ignored directories to reduce amount of os.Stat(...) calls?
		rel := strings.TrimPrefix(path, root)
		rel = strings.Trim(rel, string(os.PathSeparator))
		if rel == "" {
			return false
		}
		parts := strings.Split(rel, string(os.PathSeparator))
		if len(parts) == 0 {
			return false
		}

		var curr string
		for i := 0; i < len(parts); i++ {
			dir := parts[i]
			switch dir[0] {
			case '.', '_':
				return true
			}
			if dir == "testdata" {
				return true
			}

			curr = filepath.Join(curr, dir)
			if _, err := os.Stat(filepath.Join(root, curr, "go.mod")); err == nil {
				return true
			}
		}
		return false
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
