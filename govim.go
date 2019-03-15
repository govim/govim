// Package govim implements a Vim8 channel-based plugin host that can be used to write plugins.
package govim

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	ERROR = "ERROR"
)

type callbackResp struct {
	errString string
	val       json.RawMessage
}

type Govim struct {
	in  *json.Decoder
	out *json.Encoder
	err *os.File

	funcHandlers     map[string]interface{}
	funcHandlersLock sync.Mutex

	// callCallbackNextID represents the next ID to use in a call to the Vim
	// channel handler. This then allows us to direct the response.
	callCallbackNextID int
	callbackResps      map[int]chan callbackResp
	callbackRespsLock  sync.Mutex

	// channelCmdNextID reprents the next ID to use for a channel command
	// that will give us a response
	channelCmdNextID int
	channelCmds      map[int]chan json.RawMessage
	channelCmdsLock  sync.Mutex
}

func NewGoVim(in io.Reader, out io.Writer) (*Govim, error) {
	// TODO fix this
	log, err := os.OpenFile(filepath.Join(os.TempDir(), "govim.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	return &Govim{
		in:  json.NewDecoder(in),
		out: json.NewEncoder(out),
		err: log,

		funcHandlers: make(map[string]interface{}),

		callCallbackNextID: 1,
		callbackResps:      make(map[int]chan callbackResp),
	}, nil
}

// funcHandler returns the
func (g *Govim) funcHandler(name string) interface{} {
	g.funcHandlersLock.Lock()
	defer g.funcHandlersLock.Unlock()
	f, ok := g.funcHandlers[name]
	if !ok {
		g.errProto("tried to invoke %v but no function defined", name)
	}
	return f
}

type vimFunction func(args ...json.RawMessage) (interface{}, error)
type vimRangeFunction func(line1, line2 int, args ...json.RawMessage) (interface{}, error)

// Run is a user-friendly run wrapper
func (g *Govim) Run() error {
	return g.DoProto(g.run)
}

// run is the main loop that handles call from Vim
func (g *Govim) run() {
	for {
		g.logf("run: waiting to read a JSON message\n")
		id, msg := g.readJSONMsg()
		g.logf("run: got a message: %v: %s\n", id, msg)
		args := g.parseJSONArgSlice(msg)
		typ := g.parseString(args[0])
		args = args[1:]
		switch typ {
		case "callback":
			// This case is a "return" from a call to callCallback. Format of args
			// will be [id, [string, val]]
			id := g.parseInt(args[0])
			resp := g.parseJSONArgSlice(args[1])
			msg := g.parseString(resp[0])
			var val json.RawMessage
			if len(resp) == 2 {
				val = resp[1]
			}
			g.logf("got a callback response: [%v, %s]\n", id, args[1])
			g.callbackRespsLock.Lock()
			ch, ok := g.callbackResps[id]
			delete(g.callbackResps, id)
			g.callbackRespsLock.Unlock()
			if !ok {
				g.errProto("run: received response for callback %v, but not response chan defined", id)
			}
			go func() {
				ch <- callbackResp{
					errString: msg,
					val:       val,
				}
			}()
		case "function":
			fname := g.parseString(args[0])
			fargs := args[1:]
			g.logf("got a function call: %v, %v\n", fname, fargs)
			f := g.funcHandler(fname)
			var line1, line2 int
			var call func() (interface{}, error)
			switch f := f.(type) {
			case vimRangeFunction:
				line1 = g.parseInt(fargs[0])
				line2 = g.parseInt(fargs[1])
				fargs = fargs[2:]
				call = func() (interface{}, error) {
					return f(line1, line2, fargs...)
				}
			case vimFunction:
				call = func() (interface{}, error) {
					return f(fargs...)
				}
			}
			go func() {
				resp := [2]interface{}{"", ""}
				if res, err := call(); err != nil {
					errStr := fmt.Sprintf("got error whilst handling %v: %v", fname, err)
					g.errorf(errStr)
					resp[0] = errStr
				} else {
					resp[1] = res
				}
				g.sendJSONMsg(id, resp)
			}()
		}
	}
}

// DefineFunction defines the named function in Vim. name must begin with a capital
// letter. params is the parameters that will be used in the Vim function delcaration.
// If params is nil, then "..." is assumed.
func (g *Govim) DefineFunction(name string, params []string, f vimFunction) error {
	g.logf("DefineFunction: %v, %v\n", name, params)
	var err error
	if name == "" {
		return fmt.Errorf("function name must not be empty")
	}
	r, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsUpper(r) {
		return fmt.Errorf("function name %q must begin with a capital letter", name)
	}
	g.funcHandlersLock.Lock()
	if _, ok := g.funcHandlers[name]; ok {
		g.funcHandlersLock.Unlock()
		return fmt.Errorf("function already defined with name %q", name)
	}
	g.funcHandlers[name] = f
	g.funcHandlersLock.Unlock()
	if params == nil {
		params = []string{"..."}
	}
	args := []interface{}{name, params}
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback("function", args...)
	})
	if err != nil {
		return err
	}
	if resp := <-ch; resp.errString != "" {
		return fmt.Errorf("failed to define %q in Vim: %v", name, resp.errString)
	}
	return nil
}

// ChannelRedraw performs a redraw in Vim
func (g *Govim) ChannelRedraw(force bool) error {
	g.logf("ChannelRedraw: %v\n", force)
	var err error
	var sForce string
	if force {
		sForce = "force"
	}
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback("redraw", sForce)
	})
	if err != nil {
		return err
	}
	if resp := <-ch; resp.errString != "" {
		return fmt.Errorf("failed to redraw (force = %v) in Vim: %v", force, resp.errString)
	}
	return nil
}

// ChannelEx executes a ex command in Vim
func (g *Govim) ChannelEx(expr string) error {
	g.logf("ChannelEx: %v\n", expr)
	var err error
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback("ex", expr)
	})
	if err != nil {
		return err
	}
	if resp := <-ch; resp.errString != "" {
		return fmt.Errorf("failed to ex(%v) in Vim: %v", expr, resp.errString)
	}
	return nil
}

// ChannelNormal run a command in normal mode in Vim
func (g *Govim) ChannelNormal(expr string) error {
	g.logf("ChannelNormal: %v\n", expr)
	var err error
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback("normal", expr)
	})
	if err != nil {
		return err
	}
	if resp := <-ch; resp.errString != "" {
		return fmt.Errorf("failed to normal(%v) in Vim: %v", expr, resp.errString)
	}
	return nil
}

// ChannelExpr evaluates and returns the result of expr in Vim
func (g *Govim) ChannelExpr(expr string) (interface{}, error) {
	g.logf("ChannelExpr: %v\n", expr)
	var err error
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback("expr", expr)
	})
	if err != nil {
		return nil, err
	}
	resp := <-ch
	if resp.errString != "" {
		return nil, fmt.Errorf("failed to expr(%v) in Vim: %v", expr, resp.errString)
	}
	return resp.val, nil
}

// ChannelCall evaluates and returns the result of call in Vim
func (g *Govim) ChannelCall(fn string, args ...interface{}) (interface{}, error) {
	args = append([]interface{}{fn}, args...)
	g.logf("ChannelCall: %v\n", args...)
	var err error
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback("call", args...)
	})
	if err != nil {
		return nil, err
	}
	resp := <-ch
	if resp.errString != "" {
		return nil, fmt.Errorf("failed to call(%v) in Vim: %v", args, resp.errString)
	}
	return resp.val, nil
}

// DoProto is used as a wrapper around function calls that jump the "interface"
// between the user and protocol aspects of govim.
func (g *Govim) DoProto(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r := r.(type) {
			case errProto:
				if r.underlying == io.EOF {
					g.logf("closing connection\n")
					return
				}
				err = r
			case error:
				err = r
			default:
				panic(r)
			}
		}
	}()
	f()
	return
}

// callCallback is a low-level protocol primitive for making a call to the channel
// defined handler in Vim. The Vim handler switches on typ. The Vim handler does
// not return a value, instead we acknowledge success by sending a zero-length
// string.
func (g *Govim) callCallback(typ string, vs ...interface{}) chan callbackResp {
	g.callbackRespsLock.Lock()
	id := g.callCallbackNextID
	g.callCallbackNextID++
	ch := make(chan callbackResp)
	g.callbackResps[id] = ch
	g.callbackRespsLock.Unlock()
	args := []interface{}{id, typ}
	args = append(args, vs...)
	g.sendJSONMsg(0, args)
	return ch
}

// readJSONMsg is a low-level protocol primitive for reading a JSON msg sent by Vim.
// There is more structure to the messages that we receive, hence we can be
// more specific in our return type. See
// https://vimhelp.org/channel.txt.html#channel-use for more details.
func (g *Govim) readJSONMsg() (int, json.RawMessage) {
	var msg [2]json.RawMessage
	if err := g.in.Decode(&msg); err != nil {
		if err == io.EOF {
			panic(errProto{underlying: err})
		}
		g.errProto("failed to read JSON msg: %v", err)
	}
	i := g.parseInt(msg[0])
	return i, msg[1]
}

// parseJSONArgSlice is a low-level protocol primitive for parsing a slice of
// raw encoded JSON values
func (g *Govim) parseJSONArgSlice(m json.RawMessage) []json.RawMessage {
	var i []json.RawMessage
	g.decodeJSON(m, &i)
	return i
}

// parseString is a low-level protocol primtive for parsing a string from a
// raw encoded JSON value
func (g *Govim) parseString(m json.RawMessage) string {
	var s string
	g.decodeJSON(m, &s)
	return s
}

// parseInt is a low-level protocol primtive for parsing an int from a
// raw encoded JSON value
func (g *Govim) parseInt(m json.RawMessage) int {
	var i int
	g.decodeJSON(m, &i)
	return i
}

// sendJSONMsg is a low-level protocol primitive for sending a JSON msg that will be
// understood by Vim. See https://vimhelp.org/channel.txt.html#channel-use
func (g *Govim) sendJSONMsg(p1, p2 interface{}) {
	g.logf("sendJSONMsg: [%v, %v]\n", p1, p2)
	msg := [2]interface{}{p1, p2}
	if err := g.out.Encode(msg); err != nil {
		g.errProto("failed to send msg: %v", err)
	}
}

// decodeJSON is a low-level protocol primitive for decoding a JSON value.
func (g *Govim) decodeJSON(m json.RawMessage, i interface{}) {
	err := json.Unmarshal(m, i)
	if err != nil {
		g.errProto("failed to decode JSON into type %T: %v", i, err)
	}
}

// encodeJSON is a low-level protocol primitive used for encoding a JSON value
func (g *Govim) encodeJSON(i interface{}) json.RawMessage {
	bs, err := json.Marshal(i)
	if err != nil {
		g.errProto("failed to JSON encode %v: %v", i, err)
	}
	return bs
}

func (g *Govim) errProto(format string, args ...interface{}) {
	panic(errProto{
		underlying: fmt.Errorf(format, args...),
	})
}

// errorf is a means of raising an error that will be logged. i.e. it does not
// represent a protocol error, and Vim + govim _might_ be able to recover from
// this situation.
func (g *Govim) errorf(format string, args ...interface{}) {
	defer func() {
		fmt.Fprintln(g.err, recover())
	}()
	panic(fmt.Errorf(format, args...))
}

func (g *Govim) logf(format string, args ...interface{}) {
	fmt.Fprintf(g.err, format, args...)
}

type errProto struct {
	underlying error
}

func (e errProto) Error() string {
	return fmt.Sprintf("protocol error: %v", e.underlying)
}
