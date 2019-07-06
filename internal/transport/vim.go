package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

type vimTransport struct {
	out *json.Encoder
	log io.Writer
}

func NewVimTransport(in io.Reader, out io.Writer, log io.Writer) Transport {
	return &vimTransport{
		out: json.NewEncoder(out),
		log: log,
	}
}

func (v *vimTransport) Start() error {
	return nil
}
func (v *vimTransport) Close() error {
	return nil
}

func (v *vimTransport) Loaded() chan struct{} {
	return nil
}

func (v *vimTransport) Initialized() chan struct{} {
	return nil
}

func (v *vimTransport) IsShutdown() chan struct{} {
	return nil
}

func (v *vimTransport) Receive() (json.RawMessage, error) {
	return nil, nil
}

func (v *vimTransport) Send(callback Callback, msgType string, params ...interface{}) error {
	return nil
}

// sendJSONMsg is a low-level protocol primitive for sending a JSON msg that will be
// understood by Vim. See https://vimhelp.org/channel.txt.html#channel-use
func (v *vimTransport) SendJSON(p1, p2 interface{}, ps ...interface{}) {
	msg := []interface{}{p1, p2}
	msg = append(msg, ps...)
	// TODO could use a multi-writer here
	logMsg, err := json.Marshal(msg)
	if err != nil {
		v.errProto("failed to create log message: %v", err)
	}
	v.logVimEventf("sendJSONMsg: %s\n", logMsg)
	if err := v.out.Encode(msg); err != nil {
		panic(ErrShuttingDown)
	}
}

func (v *vimTransport) errProto(format string, args ...interface{}) {
	panic(errProto{
		underlying: fmt.Errorf(format, args...),
	})
}
func (v *vimTransport) logVimEventf(format string, args ...interface{}) {
	v.Logf("vim start =======================\n"+format+"vim end =======================\n", args...)
}

func (v *vimTransport) Logf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	if s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	t := time.Now().Format("2006-01-02T15:04:05.000000")
	s = strings.Replace(s, "\n", "\n"+t+": ", -1)
	fmt.Fprint(v.log, t+": "+s+"\n")
}
