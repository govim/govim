// Package govim implements a Vim8 channel-based plugin host that can be used to write plugins.
package govim

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/myitcv/govim/internal/queue"
	"gopkg.in/tomb.v2"
)

//go:generate gobin -m -run golang.org/x/tools/cmd/stringer -type=GenAttr,Complete,Range,Event,NArgs -linecomment -output gen_stringers_stringer.go

const (
	funcHandlePref     = "function:"
	commHandlePref     = "command:"
	autoCommHandlePref = "autocommand:"

	sysFuncPref = "govim:"
)

type callbackResp struct {
	errString string
	val       json.RawMessage
}

type Plugin interface {
	Init(Govim) error
	Shutdown() error
}

type Govim interface {
	// ChannelEx executes a ex command in Vim
	ChannelEx(expr string) error

	// ChannelExpr evaluates and returns the result of expr in Vim
	ChannelExpr(expr string) (json.RawMessage, error)

	// ChannelNormal run a command in normal mode in Vim
	ChannelNormal(expr string) error

	// ChannelCall evaluates and returns the result of call in Vim
	ChannelCall(fn string, args ...interface{}) (json.RawMessage, error)

	// ChannelRedraw performs a redraw in Vim
	ChannelRedraw(force bool) error

	// DefineFunction defines the named function in Vim. name must begin with a capital
	// letter. params is the parameters that will be used in the Vim function delcaration.
	// If params is nil, then "..." is assumed.
	DefineFunction(name string, params []string, f VimFunction) error

	// DefineRangeFunction defines the named function as range-based in Vim. name
	// must begin with a capital letter. params is the parameters that will be used
	// in the Vim function delcaration.  If params is nil, then "..." is assumed.
	DefineRangeFunction(name string, params []string, f VimRangeFunction) error

	// DefineCommand defines the named command in Vim. name must begin with a
	// capital letter. attrs is a series of attributes for the command; see :help
	// E174 in Vim for more details.
	DefineCommand(name string, f VimCommandFunction, attrs ...CommAttr) error

	// DefineAutoCommand defines an autocmd for events for files matching patterns.
	DefineAutoCommand(group string, events Events, patts Patterns, nested bool, f VimAutoCommandFunction) error

	// Run is a user-friendly run wrapper
	Run() error

	// DoProto is used as a wrapper around function calls that jump the "interface"
	// between the user and protocol aspects of govim.
	DoProto(f func()) error

	// SubOnViewportChange creates a subscription to the OnViewportChange event
	// exposed by Govim
	SubOnViewportChange(f func(Viewport) error) *OnViewportChangeSub

	// UnsubOnViewportChange removes a subscription to the OnViewportChange event.
	// It panics if sub is not an active subscription.
	UnsubOnViewportChange(sub *OnViewportChangeSub)

	ToggleOnViewportChange()

	// Viewport returns the active Vim viewport
	Viewport() Viewport

	Errorf(format string, args ...interface{})
	Logf(format string, args ...interface{})

	Sync() Govim
}

type govimImpl struct {
	in  *json.Decoder
	out *json.Encoder
	log io.Writer

	funcHandlers     map[string]interface{}
	funcHandlersLock sync.Mutex

	plugin Plugin

	flushEvents chan struct{}

	// callCallbackNextID represents the next ID to use in a call to the Vim
	// channel handler. This then allows us to direct the response.
	callCallbackNextID int
	callbackResps      map[int]chan callbackResp
	callbackRespsLock  sync.Mutex

	autocmdNextID int

	loaded chan bool

	currViewport             Viewport
	viewportLock             sync.Mutex
	onViewportChangeSubs     []*OnViewportChangeSub
	onViewportChangeSubsLock sync.Mutex

	tomb tomb.Tomb
}

func NewGovim(plug Plugin, in io.Reader, out io.Writer, log io.Writer) (Govim, error) {
	g := &govimImpl{
		in:  json.NewDecoder(in),
		out: json.NewEncoder(out),
		log: log,

		funcHandlers: make(map[string]interface{}),

		plugin: plug,

		loaded: make(chan bool),

		flushEvents: make(chan struct{}),

		callCallbackNextID: 1,
		callbackResps:      make(map[int]chan callbackResp),
	}

	return g, nil
}

func (g *govimImpl) Sync() Govim {
	return userQInst{
		govimImpl: g,
	}
}

func (g *govimImpl) load() error {
	// g.sendJSONMsg("call", "s:something", []interface{}{}, 15)
	g.funcHandlersLock.Lock()
	g.funcHandlers[sysFuncOnViewportChange] = internalFunction(g.onViewportChange)
	g.funcHandlersLock.Unlock()
	select {
	case <-g.tomb.Dying():
		return tomb.ErrDying
	case resp := <-g.callCallback("loaded"):
		if resp.errString != "" {
			return fmt.Errorf("failed to signal loaded to Vim: %v", resp.errString)
		}
	}
	close(g.loaded)

	if g.plugin != nil {
		var err error
		perr := g.DoProto(func() {
			err = g.plugin.Init(g)
		})
		if perr != nil {
			return perr
		}
		if err != nil {
			return err
		}
	}

	select {
	case <-g.tomb.Dying():
		return tomb.ErrDying
	case resp := <-g.callCallback("initcomplete"):
		if resp.errString != "" {
			return fmt.Errorf("failed to signal initcomplete to Vim: %v", resp.errString)
		}
	}

	return nil
}

// funcHandler returns the
func (g *govimImpl) funcHandler(name string) (string, interface{}) {
	g.funcHandlersLock.Lock()
	defer g.funcHandlersLock.Unlock()
	f, ok := g.funcHandlers[name]
	if !ok {
		g.errProto("tried to invoke %v but no function defined", name)
	}
	return strings.TrimPrefix(name, funcHandlePref), f
}

type internalFunction func(args ...json.RawMessage) (interface{}, error)

// VimFunction is the signature of a callback from a defined function
type VimFunction func(g Govim, args ...json.RawMessage) (interface{}, error)

// VimRangeFunction is the signature of a callback from a defined range-based
// function
type VimRangeFunction func(g Govim, line1, line2 int, args ...json.RawMessage) (interface{}, error)

// VimCommandFunction is the signature of a callback from a defined command
type VimCommandFunction func(g Govim, flags CommandFlags, args ...string) error

// VimAutoCommandFunction is the signature of a callback from a defined autocmd
type VimAutoCommandFunction func(g Govim) error

func (g *govimImpl) Run() error {
	err := g.DoProto(g.run)
	g.tomb.Kill(err)
	if err := g.tomb.Wait(); err != nil {
		return err
	}
	if g.plugin != nil {
		return g.plugin.Shutdown()
	}
	return nil
}

// run is the main loop that handles call from Vim
func (g *govimImpl) run() {
	userQ := queue.NewQueue()
	eventQ := queue.NewQueue()

	// TODO we do not properly handle errors in init functions
	// To properly handle it we will need to send a kill signal
	// to Vim to then close the channel. For now it happens will
	// will likely get a tomb error below
	g.runUserQ(userQ)
	g.runEventQ(eventQ)

	g.tomb.Go(g.load)

	// the read loop
	for {
		g.Logf("run: waiting to read a JSON message\n")
		id, msg := g.readJSONMsg()
		g.Logf("run: got a message: %v: %s\n", id, msg)
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
			g.Logf("got a callback response: [%v, %s]\n", id, args[1])
			g.callbackRespsLock.Lock()
			ch, ok := g.callbackResps[id]
			delete(g.callbackResps, id)
			g.callbackRespsLock.Unlock()
			if !ok {
				g.errProto("run: received response for callback %v, but not response chan defined", id)
			}
			g.tomb.Go(func() error {
				resp := callbackResp{
					errString: msg,
					val:       val,
				}
				select {
				case ch <- resp:
				case <-g.tomb.Dying():
					return tomb.ErrDying
				}
				return nil
			})
		case "function":
			fname := g.parseString(args[0])
			fargs := args[1:]
			g.Logf("got a function call: %v(%s)\n", fname, fargs)
			fname, f := g.funcHandler(fname)
			var line1, line2 int
			var call func() (interface{}, error)

			theQueue := userQ

			switch f := f.(type) {
			case internalFunction:
				theQueue = eventQ
				fargs = g.parseJSONArgSlice(fargs[0])
				call = func() (interface{}, error) {
					return f(fargs...)
				}
			case VimRangeFunction:
				line1 = g.parseInt(fargs[0])
				line2 = g.parseInt(fargs[1])
				fargs = g.parseJSONArgSlice(fargs[2])
				call = func() (interface{}, error) {
					return f(userQInst{g}, line1, line2, fargs...)
				}
			case VimFunction:
				fargs = g.parseJSONArgSlice(fargs[0])
				call = func() (interface{}, error) {
					return f(userQInst{g}, fargs...)
				}
			case VimCommandFunction:
				var flagVals CommandFlags
				g.decodeJSON(fargs[0], &flagVals)
				var args []string
				for _, f := range fargs[1:] {
					args = append(args, g.parseString(f))
				}
				call = func() (interface{}, error) {
					err := f(userQInst{g}, flagVals, args...)
					return nil, err
				}
			case VimAutoCommandFunction:
				theQueue = eventQ
				call = func() (interface{}, error) {
					err := f(g)
					return nil, err
				}
			default:
				g.Errorf("unknown function type for %v %T", fname, f)
			}
			theQueue.Add(func() {
				resp := [2]interface{}{"", ""}
				var res interface{}
				var err error
				func() {
					defer func() {
						if r := recover(); r != nil {
							stack := make([]byte, 20*(1<<10))
							l := runtime.Stack(stack, true)
							err = fmt.Errorf("caught panic: %v\n%s", r, stack[:l])
						}
					}()
					res, err = call()
				}()
				if err != nil {
					errStr := fmt.Sprintf("got error whilst handling %v: %v", fname, err)
					g.Logf(errStr)
					resp[0] = errStr
				} else {
					resp[1] = res
				}
				g.sendJSONMsg(id, resp)
			})
		case "log":
			var is []interface{}
			for _, a := range args {
				var i interface{}
				g.decodeJSON(a, &i)
				is = append(is, i)
			}
			fmt.Fprintln(g.log, is...)
		}
	}
}

func (g *govimImpl) runUserQ(q *queue.Queue) {
	g.tomb.Go(func() error {
		for {
			// drain the event queue first
			select {
			case g.flushEvents <- struct{}{}:
				<-g.flushEvents
				continue
			case <-g.tomb.Dying():
				return tomb.ErrDying
			default:
			}
			select {
			case g.flushEvents <- struct{}{}:
				<-g.flushEvents
				continue
			case <-g.tomb.Dying():
				return tomb.ErrDying
			case <-q.GotWork():
				if f, ok := q.Get(); ok {
					f()
				}
			}
		}
	})
}

func (g *govimImpl) runEventQ(q *queue.Queue) {
	g.tomb.Go(func() error {
		for {
			// drain the event queue first
			select {
			case <-q.GotWork():
			case <-g.tomb.Dying():
				return tomb.ErrDying
			}
			select {
			case <-g.flushEvents:
				for {
					work, ok := q.Get()
					if !ok {
						break
					}
					work()
				}
				g.flushEvents <- struct{}{}
			case <-g.tomb.Dying():
				return tomb.ErrDying
			}
		}
	})
}

func (g *govimImpl) DefineFunction(name string, params []string, f VimFunction) error {
	<-g.loaded
	g.Logf("DefineFunction: %v, %v\n", name, params)
	return g.defineFunction(false, name, params, f)
}

func (g *govimImpl) DefineRangeFunction(name string, params []string, f VimRangeFunction) error {
	<-g.loaded
	g.Logf("DefineRangeFunction: %v, %v\n", name, params)
	return g.defineFunction(true, name, params, f)
}

func (g *govimImpl) defineFunction(isRange bool, name string, params []string, f interface{}) error {
	var err error
	if name == "" {
		return fmt.Errorf("function name must not be empty")
	}
	r, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsUpper(r) {
		return fmt.Errorf("function name %q must begin with a capital letter", name)
	}
	funcHandle := funcHandlePref + name
	g.funcHandlersLock.Lock()
	if _, ok := g.funcHandlers[funcHandle]; ok {
		g.funcHandlersLock.Unlock()
		return fmt.Errorf("function already defined with name %q", name)
	}
	g.funcHandlers[funcHandle] = f
	g.funcHandlersLock.Unlock()
	if params == nil {
		params = []string{"..."}
	}
	args := []interface{}{name, params}
	callbackTyp := "function"
	if isRange {
		callbackTyp = "rangefunction"
	}
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback(callbackTyp, args...)
	})
	if err != nil {
		return err
	}
	if resp := <-ch; resp.errString != "" {
		return fmt.Errorf("failed to define %q in Vim: %v", name, resp.errString)
	}
	return nil
}

func (g *govimImpl) DefineAutoCommand(group string, events Events, patts Patterns, nested bool, f VimAutoCommandFunction) error {
	<-g.loaded
	var err error
	g.funcHandlersLock.Lock()
	funcHandle := fmt.Sprintf("%v%v", autoCommHandlePref, g.autocmdNextID)
	g.autocmdNextID++
	if _, ok := g.funcHandlers[funcHandle]; ok {
		g.funcHandlersLock.Unlock()
		return fmt.Errorf("function already defined with handler %q", funcHandle)
	}
	g.funcHandlers[funcHandle] = f
	g.funcHandlersLock.Unlock()
	var def strings.Builder
	w := func(s string) {
		def.WriteString(" " + s)
	}
	if group != "" {
		w(group)
	}
	var strEvents []string
	for _, e := range events {
		strEvents = append(strEvents, e.String())
	}
	sort.Strings(strEvents)
	w(strings.Join(strEvents, ","))
	// TODO validate patterns
	var strPatts []string
	for _, p := range patts {
		strPatts = append(strPatts, string(p))
	}
	sort.Strings(strPatts)
	w(strings.Join(strPatts, ","))
	if nested {
		w("nested")
	}
	args := []interface{}{funcHandle, def.String()}
	callbackTyp := "autocmd"
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback(callbackTyp, args...)
	})
	if err != nil {
		return err
	}
	if resp := <-ch; resp.errString != "" {
		return fmt.Errorf("failed to define autocmd %q in Vim: %v", def.String(), resp.errString)
	}
	return nil
}

func (g *govimImpl) DefineCommand(name string, f VimCommandFunction, attrs ...CommAttr) error {
	<-g.loaded
	var err error
	if name == "" {
		return fmt.Errorf("command name must not be empty")
	}
	r, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsUpper(r) {
		return fmt.Errorf("command name %q must begin with a capital letter", name)
	}
	funcHandle := commHandlePref + name
	g.funcHandlersLock.Lock()
	if _, ok := g.funcHandlers[funcHandle]; ok {
		g.funcHandlersLock.Unlock()
		return fmt.Errorf("command already defined with name %q", name)
	}
	g.funcHandlers[funcHandle] = f
	g.funcHandlersLock.Unlock()
	var nargsFlag *NArgs
	var rangeFlag *Range
	var rangeNFlag *RangeN
	var countNFlag *CountN
	var completeFlag *CommAttr
	genAttrs := make(map[CommAttr]bool)
	for _, iattr := range attrs {
		switch attr := iattr.(type) {
		case NArgs:
			switch attr {
			case NArgs0, NArgs1, NArgsZeroOrMore, NArgsZeroOrOne, NArgsOneOrMore:
			default:
				return fmt.Errorf("unknown NArgs value")
			}
			if nargsFlag != nil && attr != *nargsFlag {
				return fmt.Errorf("multiple nargs flags")
			}
			nargsFlag = &attr
		case Range:
			switch attr {
			case RangeLine, RangeFile:
			default:
				return fmt.Errorf("unknown Range value")
			}
			if rangeFlag != nil && *rangeFlag != attr || rangeNFlag != nil {
				return fmt.Errorf("multiple range flags")
			}
			if countNFlag != nil {
				return fmt.Errorf("range and count flags are mutually exclusive")
			}
			rangeFlag = &attr
		case RangeN:
			if rangeNFlag != nil && *rangeNFlag != attr || rangeFlag != nil {
				return fmt.Errorf("multiple range flags")
			}
			if countNFlag != nil {
				return fmt.Errorf("range and count flags are mutually exclusive")
			}
			rangeNFlag = &attr
		case CountN:
			if countNFlag != nil && *countNFlag != attr {
				return fmt.Errorf("multiple count flags")
			}
			if rangeFlag != nil || rangeNFlag != nil {
				return fmt.Errorf("range and count flags are mutually exclusive")
			}
			countNFlag = &attr
		case Complete:
			if completeFlag != nil && *completeFlag != attr {
				return fmt.Errorf("multiple complete flags")
			}
			completeFlag = &iattr
		case CompleteCustom:
			if completeFlag != nil && *completeFlag != attr {
				return fmt.Errorf("multiple complete flags")
			}
			completeFlag = &iattr
		case CompleteCustomList:
			if completeFlag != nil && *completeFlag != attr {
				return fmt.Errorf("multiple complete flags")
			}
			completeFlag = &iattr
		case GenAttr:
			switch attr {
			case AttrBang, AttrRegister, AttrBuffer, AttrBar:
				genAttrs[attr] = true
			default:
				return fmt.Errorf("unknown GenAttr value")
			}
		}
	}
	attrMap := make(map[string]interface{})
	if nargsFlag != nil {
		attrMap["nargs"] = nargsFlag.String()
	}
	if rangeFlag != nil {
		attrMap["range"] = rangeFlag.String()
	}
	if rangeNFlag != nil {
		attrMap["range"] = rangeNFlag.String()
	}
	if countNFlag != nil {
		attrMap["count"] = countNFlag.String()
	}
	if completeFlag != nil {
		attrMap["complete"] = (*completeFlag).String()
	}
	if len(genAttrs) > 0 {
		var attrs []string
		for k := range genAttrs {
			attrs = append(attrs, k.String())
		}
		sort.Strings(attrs)
		attrMap["general"] = attrs
	}
	args := []interface{}{name, attrMap}
	var ch chan callbackResp
	err = g.DoProto(func() {
		ch = g.callCallback("command", args...)
	})
	if err != nil {
		return err
	}
	if resp := <-ch; resp.errString != "" {
		return fmt.Errorf("failed to define %q in Vim: %v", name, resp.errString)
	}
	return nil
}

// callCallback is a low-level protocol primitive for making a call to the
// channel defined handler in Vim. The Vim handler switches on typ. The Vim
// handler does not return a value, instead it will acknowledge success by
// sending a zero-length string.
func (g *govimImpl) callCallback(typ string, vs ...interface{}) chan callbackResp {
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
func (g *govimImpl) readJSONMsg() (int, json.RawMessage) {
	var msg [2]json.RawMessage
	if err := g.in.Decode(&msg); err != nil {
		if err == io.EOF {
			// explicitly setting underlying here
			panic(errProto{underlying: err})
		}
		g.errProto("failed to read JSON msg: %v", err)
	}
	i := g.parseInt(msg[0])
	return i, msg[1]
}

// parseJSONArgSlice is a low-level protocol primitive for parsing a slice of
// raw encoded JSON values
func (g *govimImpl) parseJSONArgSlice(m json.RawMessage) []json.RawMessage {
	var i []json.RawMessage
	g.decodeJSON(m, &i)
	return i
}

// parseString is a low-level protocol primtive for parsing a string from a
// raw encoded JSON value
func (g *govimImpl) parseString(m json.RawMessage) string {
	var s string
	g.decodeJSON(m, &s)
	return s
}

// parseInt is a low-level protocol primtive for parsing an int from a
// raw encoded JSON value
func (g *govimImpl) parseInt(m json.RawMessage) int {
	var i int
	g.decodeJSON(m, &i)
	return i
}

// sendJSONMsg is a low-level protocol primitive for sending a JSON msg that will be
// understood by Vim. See https://vimhelp.org/channel.txt.html#channel-use
func (g *govimImpl) sendJSONMsg(p1, p2 interface{}, ps ...interface{}) {
	msg := []interface{}{p1, p2}
	msg = append(msg, ps...)
	g.Logf("sendJSONMsg: %v\n", msg)
	if err := g.out.Encode(msg); err != nil {
		g.errProto("failed to send msg: %v", err)
	}
}

// decodeJSON is a low-level protocol primitive for decoding a JSON value.
func (g *govimImpl) decodeJSON(m json.RawMessage, i interface{}) {
	err := json.Unmarshal(m, i)
	if err != nil {
		g.errProto("failed to decode JSON into type %T: %v", i, err)
	}
}

func (g *govimImpl) errProto(format string, args ...interface{}) {
	panic(errProto{
		underlying: fmt.Errorf(format, args...),
	})
}

// Errorf is a means of raising an error that will be logged, and the govim
// instance will then effectively "stop".
func (g *govimImpl) Errorf(format string, args ...interface{}) {
	b := make([]byte, (1<<10)*10)
	runtime.Stack(b, true)
	args = append([]interface{}{}, args...)
	args = append(args, b)
	g.tomb.Kill(fmt.Errorf(format+"\n%s", args...))
}

func (g *govimImpl) Logf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	if s[len(s)-1] != '\n' {
		s += "\n"
	}
	fmt.Fprint(g.log, s)
}

type errProto struct {
	underlying error
}

func (e errProto) Error() string {
	return fmt.Sprintf("protocol error: %v", e.underlying)
}
