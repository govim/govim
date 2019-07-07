package transport

import (
	"encoding/json"
	"fmt"
	"github.com/myitcv/govim/internal/queue"
	"gopkg.in/tomb.v2"
	"io"
	"strings"
	"sync"
	"time"
)

type ResponseCallback func(p2 interface{}, ps ...interface{}) error
type FuncHandler func(args []json.RawMessage, callback ResponseCallback) error

type ErrTransportClosing struct {
	underlying error
}

func (e ErrTransportClosing) Error() string {
	return fmt.Sprintf("transport closing: %v", e.underlying)
}

type vimTransport struct {
	in  *json.Decoder
	out *json.Encoder
	log io.Writer

	// callVimNextID represents the next ID to use in a call to the Vim
	// channel handler. This then allows us to direct the response.
	callVimNextID     int
	callbackResps     map[int]Callback
	callbackRespsLock sync.Mutex

	queuer      queue.Queuer
	funcHandler FuncHandler
	tomb        tomb.Tomb
}

func NewVimTransport(in io.Reader, out io.Writer, log io.Writer, queuer queue.Queuer, funcHandler FuncHandler) Transport {
	return &vimTransport{
		in:  json.NewDecoder(in),
		out: json.NewEncoder(out),
		log: log,

		callVimNextID: 1,
		callbackResps: make(map[int]Callback),

		queuer:      queuer,
		funcHandler: funcHandler,
	}
}

func (v *vimTransport) Start() error {
	v.tomb.Go(v.run)
	<-v.tomb.Dying()
	return v.tomb.Err()
}

func (v *vimTransport) run() error {
	for {
		v.Logf("run: waiting to read a JSON message\n")
		if err := v.Read(); err != nil {
			switch err.(type) {
			case ErrTransportClosing:
				return err
			default:
				panic(fmt.Errorf("error during read: %v", err))
				// v.Logf("error during read: %v", err)
			}
		}
	}
	return nil
}

func (v *vimTransport) Close() error {
	v.tomb.Kill(ErrShuttingDown)
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

func (v *vimTransport) Read() error {
	for {
		callbackID, msg, err := v.readNextJSON()
		if err != nil {
			return err
		}
		v.logVimEventf("recvJSONMsg: [%v] %s\n", callbackID, msg)
		args := v.parseJSONArgSlice(msg)
		messageType := v.parseString(args[0])
		args = args[1:]

		switch messageType {
		case "callback":
			if err := v.handleCallback(args); err != nil {
				return fmt.Errorf("error handling callback: %v", err)
			}
		case "function":
			if err := v.handleFunctionCall(args, callbackID); err != nil {
				return fmt.Errorf("error handling function call: %v", err)
			}
		case "log":
			var is []interface{}
			for _, a := range args {
				var i interface{}
				v.decodeJSON(a, &i)
				is = append(is, i)
			}
			fmt.Fprintln(v.log, is...)
		default:
			return fmt.Errorf("unrecognised messageType %s", messageType)
		}
	}
}
func (v *vimTransport) handleFunctionCall(args []json.RawMessage, callbackID int) error {
	responseCallback := func(p2 interface{}, ps ...interface{}) error {
		return v.sendJSON(callbackID, p2, ps...)
	}
	return v.funcHandler(args, responseCallback)
}

func (v *vimTransport) handleCallback(args []json.RawMessage) error {
	// This case is a "return" from a call to callVim. Format of args
	// will be [id, [string, val]]
	id := v.parseInt(args[0])
	resp := v.parseJSONArgSlice(args[1])
	msg := v.parseString(resp[0])
	var val json.RawMessage
	if len(resp) == 2 {
		val = resp[1]
	}
	toSend := CallbackResp{
		ErrString: msg,
		Val:       val,
	}
	v.callbackRespsLock.Lock()
	ch, ok := v.callbackResps[id]
	delete(v.callbackResps, id)
	v.callbackRespsLock.Unlock()
	if !ok {
		err := fmt.Errorf("run: received response for callback %v, but not response chan defined", id)
		return err
	}
	switch ch := ch.(type) {
	case ScheduledCallback:
		v.queuer.Add(func() error {
			select {
			case ch <- toSend:
			case <-v.tomb.Dying():
				return tomb.ErrDying
			}
			return nil
		})
	case UnscheduledCallback:
		v.tomb.Go(func() error {
			select {
			case ch <- toSend:
			case <-v.tomb.Dying():
				return tomb.ErrDying
			}
			return nil
		})
	default:
		panic(fmt.Errorf("unknown type of callback responser: %T", ch))
	}
	return nil
}

func (v *vimTransport) readNextJSON() (int, json.RawMessage, error) {
	var msg [2]json.RawMessage
	if err := v.in.Decode(&msg); err != nil {
		if err == io.EOF {
			// explicitly setting underlying here
			err = ErrTransportClosing{err}
			v.tomb.Kill(err)
			return 0, nil, err
		}
		return 0, nil, fmt.Errorf("failed to read JSON msg: %v", err)
	}
	callbackID := v.parseInt(msg[0])
	return callbackID, msg[1], nil
}

func (v *vimTransport) Send(callback Callback, callbackType string, params ...interface{}) error {
	v.callbackRespsLock.Lock()
	id := v.callVimNextID
	v.callVimNextID++
	v.callbackResps[id] = callback
	v.callbackRespsLock.Unlock()
	args := []interface{}{id, callbackType}
	args = append(args, params...)
	return v.sendJSON(0, args)
}

func (v *vimTransport) SendAndReceive(messageType string, args ...interface{}) (json.RawMessage, error) {
	callback := make(UnscheduledCallback)
	if err := v.Send(callback, messageType, args...); err != nil {
		// protocol level error
		return nil, err
	}

	select {
	case <-v.tomb.Dying():
		return nil, ErrShuttingDown
	case resp := <-callback:
		if resp.ErrString != "" {
			return nil, fmt.Errorf("vim error: %s", resp.ErrString)
		}
		return resp.Val, nil
	}
}

func (v *vimTransport) SendAndReceiveAsync(messageType string, args ...interface{}) (ScheduledCallback, error) {
	callback := make(ScheduledCallback)
	if err := v.Send(callback, messageType, args...); err != nil {
		// protocol level error
		return nil, err
	}
	return callback, nil
}

// sendJSONMsg is a low-level protocol primitive for sending a JSON msg that will be
// understood by Vim. See https://vimhelp.org/channel.txt.html#channel-use
func (v *vimTransport) sendJSON(p1, p2 interface{}, ps ...interface{}) error {
	msg := []interface{}{p1, p2}
	msg = append(msg, ps...)
	// TODO could use a multi-writer here
	logMsg, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to create log message: %v", err)
	}
	v.logVimEventf("sendJSONMsg: %s\n", logMsg)
	if err := v.out.Encode(msg); err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}
	return nil
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

// parseJSONArgSlice is a low-level protocol primitive for parsing a slice of
// raw encoded JSON values
func (v *vimTransport) parseJSONArgSlice(m json.RawMessage) []json.RawMessage {
	var i []json.RawMessage
	v.decodeJSON(m, &i)
	return i
}

// parseString is a low-level protocol primtive for parsing a string from a
// raw encoded JSON value
func (v *vimTransport) parseString(m json.RawMessage) string {
	var s string
	v.decodeJSON(m, &s)
	return s
}

// decodeJSON is a low-level protocol primitive for decoding a JSON value.
func (v *vimTransport) decodeJSON(m json.RawMessage, i interface{}) {
	err := json.Unmarshal(m, i)
	if err != nil {
		v.errProto("failed to decode JSON into type %T: %v", i, err)
	}
}
