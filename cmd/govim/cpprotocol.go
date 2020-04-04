package main

// TODO: perhaps all this child-parent stuff is better housed in an internal helper
// package

// Child-parent protocol
//
// The child-parent protocol (better name required) defines the process by
// which a child and parent instances of govim communicate. The principal use
// case is being able to write scripts that run against an editor instance. The
// motivating example here being fzf which uses a child instances to query
// gopls (via the parent) for session symbol information.
//
// The parent instance exposes the command that should be run in order to
// create that child instance that then connects to the parent. This is exposed
// via the GOVIMParentCommand() Vim function.
//
// TODO: expose help information for the various commands etc
//
// The child and parent communicate using a Unix domain socket. Messages on the
// wire are encoded JSON. The child starts by sending all of its non-flag args
// to the parent. The parent is then responds with pairs of data. The pair
// comprises a JSON-encoded number followed by a JSON-encoded value. The
// following list of modes is supported:
//
// 0X - X is an integer representing the exit code the client should use
// 1X - X is a string value that should be output to os.Stdout
// 2X - X is a string value that should be output to os.Stderr
// 3x - X is an interface{} value whose JSON representation should be output to os.Stdout
// 4x - X is an interface{} value whose JSON representation should be output to os.Stderr
//
// If the parent instance encounters an error writing to the child's
// connection, it assumes the child has closed the connected and it (the
// parent) continues without error.

type encodeCode int

const (
	encodeCodeExitCode encodeCode = iota
	encodeCodeRawStdout
	encodeCodeRawStderr
	encodeCodeJSONStdout
	encodeCodeJSONStderr
)

// Command is the interface implemented by parent (sub) comamnds
type Command interface {
	// Run is the implementation of the command. A Command is responseible for
	// encoding its response (through sub commands) to the child.  This is
	// typically done by all the commands in the hierarchy embedding the root
	// *parentReq.
	Run()

	// Errorf is a convenience method for panic-ing with a knownParentError
	Errorf(format string, args ...interface{})
}
