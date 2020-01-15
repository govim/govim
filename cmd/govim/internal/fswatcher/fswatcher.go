package fswatcher

type FSWatcher struct {
	*fswatcher // os specific
}

type Event struct {
	Path string
	Op   Op
}

type Op string

const (
	OpChanged Op = "changed"
	OpRemoved Op = "removed"
	OpCreated Op = "created"
)
