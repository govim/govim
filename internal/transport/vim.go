package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

type vimTransport struct {
	in  *json.Decoder
	out *json.Encoder
	log io.Writer
}

func NewVimTransport(in io.Reader, out io.Writer, log io.Writer) Transport {
	return &vimTransport{
		in:  json.NewDecoder(in),
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

func (v *vimTransport) Read() (int, json.RawMessage, error) {
	var msg [2]json.RawMessage
	if err := v.in.Decode(&msg); err != nil {
		if err == io.EOF {
			// explicitly setting underlying here
			return 0, nil, fmt.Errorf("got EOF")
		}
		return 0, nil, fmt.Errorf("failed to read JSON msg: %v", err)
	}
	i := v.parseInt(msg[0])
	return i, msg[1], nil
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

// parseInt is a low-level protocol primtive for parsing an int from a
// raw encoded JSON value
func (v *vimTransport) parseInt(m json.RawMessage) int {
	var i int
	v.decodeJSON(m, &i)
	return i
}

// decodeJSON is a low-level protocol primitive for decoding a JSON value.
func (v *vimTransport) decodeJSON(m json.RawMessage, i interface{}) {
	err := json.Unmarshal(m, i)
	if err != nil {
		v.errProto("failed to decode JSON into type %T: %v", i, err)
	}
}
