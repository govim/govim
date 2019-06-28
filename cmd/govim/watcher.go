package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/internal/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/span"
	"github.com/myitcv/govim/cmd/govim/types"
)

type modWatcher struct {
	// We don't use the *vimstate type because we are operating outside of the Vim/vimstate
	// "thread". Perhaps slightly inefficient that we query Vim to see whether a buffer is
	// loaded or not, but that should be de minimums... and in any case this hack will soon
	// disappear
	*govimplugin

	watcher *fsnotify.Watcher

	// root is the directory root of the watch
	root string

	// watches is the set of current watches "open" in the watcher
	watches map[string]bool

	// files is a map of open files and the current version known to gopls
	// that are _not_ being handled by Vim in open buffers
	files map[string]int
}

// newWatcher returns a new watcher that will "watch" on the Go files in the
// module identified by gomodpath
func newModWatcher(plug *govimplugin, gomodpath string) (*modWatcher, error) {
	mw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create new watcher: %v", err)
	}

	dirpath := filepath.Dir(gomodpath)
	dir, err := os.Stat(dirpath)
	if err != nil || !dir.IsDir() {
		return nil, fmt.Errorf("could not resolve dir from go.mod path %v: %v", gomodpath, err)
	}

	res := &modWatcher{
		govimplugin: plug,
		watcher:     mw,
		root:        dirpath,
		watches:     make(map[string]bool),
		files:       make(map[string]int),
	}

	go res.watch()
	// fake event to kick start the watching
	mw.Events <- fsnotify.Event{
		Name: dirpath,
		Op:   fsnotify.Create,
	}

	return res, nil
}

func (m *modWatcher) close() error {
	return m.watcher.Close()
}

func (m *modWatcher) watch() {
	errf := func(format string, args ...interface{}) {
		m.Logf("**** file watcher error: "+format, args...)
	}
	infof := func(format string, args ...interface{}) {
		m.Logf("file watcher event: "+format, args...)
	}
	for {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				// watcher has been stopped?
				return
			}
			switch event.Op {
			case fsnotify.Remove, fsnotify.Rename:
				path := event.Name
				var didFind bool
				for ew := range m.watches {
					if event.Name == ew || strings.HasPrefix(ew, event.Name+string(os.PathSeparator)) {
						didFind = true
						if err := m.watcher.Remove(ew); err != nil {
							errf("failed to remove watch on %v: %v", ew, err)
						}
						infof("removed watch on %v", ew)
					}
				}
				if didFind {
					// it was probably a directory
					continue
				}
				if !ofInterest(path) {
					continue
				}
				m.Schedule(func(govim.Govim) error {
					return m.vimstate.handleEvent(event)
				})
			case fsnotify.Create, fsnotify.Write, fsnotify.Chmod:
				path := event.Name
				dirInfo, err := os.Stat(path)
				if err != nil {
					errf("failed to stat %v: %v", path, err)
					continue
				}
				if !dirInfo.IsDir() {
					// Is Vim handling this file?
					// Is this a file we care about? go.mod, *.go?
					if !ofInterest(path) {
						continue
					}
					m.Schedule(func(govim.Govim) error {
						return m.vimstate.handleEvent(event)
					})
					continue
				}

				// Walk the dir that is event.Name
				err = filepath.Walk(event.Name, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() {
						return nil
					}

					// We have a dir
					switch filepath.Base(path)[0] {
					case '.', '_':
						return nil
					}
					if path != m.root {
						// check we are not in a submodule
						if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
							return filepath.SkipDir
						}
					}
					err = m.watcher.Add(path)
					if err != nil {
						infof("added watch on %v", path)
					}
					return err
				})
				if err != nil {
					errf("failed to walk %v: %v", event.Name, err)
				}
			}
		case err, ok := <-m.watcher.Errors:
			if !ok {
				// watcher has been stopped?
				return
			}
			// TODO - handle this case better
			m.Logf("***** file watcher error: %v", err)
		}
	}
}

func ofInterest(path string) bool {
	// TODO when https://github.com/golang/go/issues/32178 is fixed re-add go.mod here
	return filepath.Ext(path) == ".go"
}

func (v *vimstate) handleEvent(event fsnotify.Event) error {
	// We are handling a filesystem event... so the best we can do is log errors
	errf := func(format string, args ...interface{}) {
		v.Logf("**** handleEvent error: "+format, args...)
	}

	path := event.Name

	for _, b := range v.buffers {
		if b.Name == path {
			// Vim is handling this file, do nothing
			v.Logf("handleEvent: Vim is in charge of %v; not handling ", event.Name)
			return nil
		}
	}

	switch event.Op {
	case fsnotify.Rename, fsnotify.Remove:
		if _, ok := v.watchedFiles[path]; !ok {
			// We saw the Rename/Remove event but nothing before
			return nil
		}
		params := &protocol.DidCloseTextDocumentParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: string(span.URI(path)),
			},
		}
		err := v.server.DidClose(context.Background(), params)
		if err != nil {
			errf("failed to call server.DidClose: %v", err)
		}
		return nil
	case fsnotify.Create, fsnotify.Chmod, fsnotify.Write:
		byts, err := ioutil.ReadFile(path)
		if err != nil {
			errf("failed to read %v: %v", path, err)
			return nil
		}
		wf, ok := v.watchedFiles[path]
		if !ok {
			wf = &types.WatchedFile{
				Path:     path,
				Contents: byts,
			}
			v.watchedFiles[path] = wf
			params := &protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{
					LanguageID: "go",
					URI:        string(wf.URI()),
					Version:    float64(0),
					Text:       string(wf.Contents),
				},
			}
			err := v.server.DidOpen(context.Background(), params)
			if err != nil {
				errf("failed to call server.DidOpen: %v", err)
			}
			v.Logf("handleEvent: handled %v", event)
			return nil
		}
		wf.Version++
		params := &protocol.DidChangeTextDocumentParams{
			TextDocument: protocol.VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: protocol.TextDocumentIdentifier{
					URI: string(wf.URI()),
				},
				Version: float64(wf.Version),
			},
			ContentChanges: []protocol.TextDocumentContentChangeEvent{
				{
					Text: string(byts),
				},
			},
		}
		err = v.server.DidChange(context.Background(), params)
		if err != nil {
			errf("failed to call server.DidChange: %v", err)
		}
		v.Logf("handleEvent: handled %v", event)
		return nil

	default:
		panic(fmt.Errorf("unknown fsnotify event type: %v", event))
	}
}
