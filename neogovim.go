package govim

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kr/pretty"
	"github.com/myitcv/govim/internal/queue"
	"github.com/neovim/go-client/nvim"

	"gopkg.in/tomb.v2"
)

type NeoGovim struct {
	*nvim.Nvim
	plugin        Plugin
	pluginErrCh   chan error
	autocmdNextID int
	autocmdLock   sync.Mutex

	funcHandlers     map[string]handler
	funcHandlersLock sync.Mutex

	initialized chan struct{}
	loaded      chan struct{}
	flushEvents chan struct{}
	shutdown    chan struct{}

	tomb tomb.Tomb

	eventQueue *queue.Queue
	version    string

	log io.Writer
}

func NewNeoGovim(plug Plugin, r io.Reader, w io.Writer, c io.Closer, log io.Writer) (*NeoGovim, error) {
	v, err := nvim.New(r, w, c, func(format string, args ...interface{}) {
		fmt.Fprintf(log, format, args...)
	})
	if err != nil {
		return nil, err
	}

	return &NeoGovim{
		Nvim:         v,
		plugin:       plug,
		log:          log,
		funcHandlers: make(map[string]handler),
		initialized:  make(chan struct{}),
		loaded:       make(chan struct{}),
		flushEvents:  make(chan struct{}),
		shutdown:     make(chan struct{}),
	}, nil
}

func waitReady(v *nvim.Nvim, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		ready := make(chan bool, 1)
		errChan := make(chan error, 1)
		go func() {
			err := v.Command(":echom 'foo'")
			if err != nil {
				errChan <- err
				return
			}
			ready <- true
		}()
		select {
		case <-ready:
			return nil
		case err := <-errChan:
			return err
		case <-time.After(10 * time.Second):
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out waiting for response")
			}
		}
	}
}

func (n *NeoGovim) load() error {
	select {
	case <-n.tomb.Dying():
		return tomb.ErrDying
	case resp := <-n.unscheduledCallCallback("loaded"):
		n.Logf("got response from loaded: '%v'", resp)
		if resp.errString != "" {
			return fmt.Errorf("failed to signal loaded to Vim: %v", resp.errString)
		}
	}
	close(n.loaded)

	n.Logf("Go version %v", runtime.Version())

	if bi, ok := debug.ReadBuildInfo(); ok {
		n.Logf("Build info: %v", pretty.Sprint(bi))
	} else {
		n.Logf("No build info available")
	}

	if n.plugin != nil {
		n.pluginErrCh = make(chan error)

		resp, err := n.Nvim.CommandOutput(":version")
		if err != nil {
			return err
		}
		versionLines := strings.Split(resp, "\n")
		n.version = versionLines[1]
		n.Logf("Loaded against %v", n.version)

		err = n.plugin.Init(n, n.pluginErrCh)
		if err != nil {
			return err
		}
		n.Logf("NeoGovim plugin initialized")

		n.tomb.Go(func() error {
			return <-n.pluginErrCh
		})
	}
	select {
	case <-n.tomb.Dying():
		return tomb.ErrDying
	case resp := <-n.unscheduledCallCallback("initcomplete"):
		if resp.errString != "" {
			return fmt.Errorf("failed to signal initcomplete to Vim: %v", resp.errString)
		}
	}

	close(n.initialized)
	return nil
}

func (n *NeoGovim) run() error {
	err := n.Nvim.RegisterHandler("callGovim", n.handleVimRequest)
	if err != nil {
		panic(err)
	}
	n.Logf("registered handler for callGovim")

	err = n.Nvim.RegisterHandler("vimShuttingDown", n.handleVimShutdown)
	if err != nil {
		panic(err)
	}
	n.Logf("registered handler for vimShuttingDown")

	n.goHandleShutdown(n.Nvim.Serve)

	n.eventQueue = queue.NewQueue()
	n.goHandleShutdown(n.runEventQueue)
	n.goHandleShutdown(n.load)
	for {
		select {
		case <-n.tomb.Dying():
			return tomb.ErrDying
		case <-n.Shutdown():
			n.tomb.Kill(ErrShuttingDown)
		}
	}
	return nil
}

// ChannelEx executes a ex command in Vim
func (n *NeoGovim) ChannelEx(expr string) error {
	err := n.Nvim.Command(expr)

	if err != nil {
		return err
	}

	return nil
}

// ChannelExpr evaluates and returns the result of expr in Vim
func (n *NeoGovim) ChannelExpr(expr string) (json.RawMessage, error) {
	var res interface{}
	err := n.Nvim.Eval(expr, &res)
	if err != nil {
		n.logVimEventf("recvJSONMsg error: %v\n", err)
		return nil, err
	}

	n.logVimEventf("recvJSONMsg: %v\n", res)
	jsonM, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	return jsonM, nil
}

// ChannelNormal run a command in normal mode in Vim
func (n *NeoGovim) ChannelNormal(expr string) error {
	expr = fmt.Sprintf("normal %s", expr)

	err := n.Nvim.Command(expr)

	if err != nil {
		return err
	}

	return nil
}

// ChannelCall evaluates and returns the result of call in Vim
func (n *NeoGovim) ChannelCall(fn string, args ...interface{}) (json.RawMessage, error) {
	strArgs := []string{}
	for _, a := range args {
		v, err := json.Marshal(a)
		if err != nil {
			return nil, err
		}
		strArgs = append(strArgs, string(v))
	}

	expr := fmt.Sprintf("echom %s(%s)", fn, strings.Join(strArgs, ", "))

	res, err := n.Nvim.CommandOutput(expr)

	if err != nil {
		return nil, err
	}

	jsonM, err := json.Marshal(string(res))
	if err != nil {
		return nil, err
	}

	return jsonM, nil
}

// ChannelRedraw performs a redraw in Vim
func (n *NeoGovim) ChannelRedraw(force bool) error {
	expr := "redraw"
	if force {
		expr += "!"
	}
	return n.ChannelEx(expr)
}

// DefineFunction defines the named function in Vim. name must begin with a capital
// letter. params is the parameters that will be used in the Vim function delcaration.
// If params is nil, then "..." is assumed.
func (n *NeoGovim) DefineFunction(name string, params []string, f VimFunction) error {
	// wrappedF := func(nv *nvim.Nvim, args []interface{}, extraArgs []interface{}) (interface{}, error) {
	// 	jsonArgs := []json.RawMessage{}
	// 	for _, a := range args {
	// 		v, err := json.Marshal(a)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		jsonArgs = append(jsonArgs, v)
	// 	}
	// 	for _, a := range extraArgs {
	// 		v, err := json.Marshal(a)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		jsonArgs = append(jsonArgs, v)
	// 	}
	// 	return f(n, jsonArgs...)
	// }
	// return n.defineFunction(false, name, params, wrappedF)
	<-n.loaded
	return n.defineFunction(false, name, params, f)
}

// DefineRangeFunction defines the named function as range-based in Vim. name
// must begin with a capital letter. params is the parameters that will be used
// in the Vim function delcaration.  If params is nil, then "..." is assumed.
func (n *NeoGovim) DefineRangeFunction(name string, params []string, f VimRangeFunction) error {
	// wrappedF := func(nv *nvim.Nvim, l1, l2 int, args ...interface{}) (interface{}, error) {
	// 	return f(n, l1, l2)
	// }
	// return n.defineFunction(true, name, params, wrappedF)
	<-n.loaded
	return n.defineFunction(true, name, params, f)
}

func (n *NeoGovim) defineFunction(isRange bool, name string, params []string, f handler) error {
	var err error
	if name == "" {
		return fmt.Errorf("function name must not be empty")
	}
	r, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsUpper(r) {
		return fmt.Errorf("function name %q must begin with a capital letter", name)
	}
	funcHandle := funcHandlePref + name
	n.funcHandlersLock.Lock()
	if _, ok := n.funcHandlers[funcHandle]; ok {
		n.funcHandlersLock.Unlock()
		return fmt.Errorf("function already defined with name %q", name)
	}
	n.funcHandlers[funcHandle] = f
	n.funcHandlersLock.Unlock()
	if params == nil {
		params = []string{"..."}
	}
	args := []interface{}{name, params}
	callbackTyp := "function"
	if isRange {
		callbackTyp = "rangefunction"
	}
	ch := make(unscheduledCallback)
	err = n.DoProto(func() error {
		return n.callVim(ch, callbackTyp, args...)
	})

	n.Logf("sent off defining function %q", name)

	return n.handleChannelError(ch, err, "failed to define %q in Vim: %v", name)
	// err = n.Nvim.RegisterHandler(funcHandle, f)
	// if err != nil {
	// 	return err
	// }

	// wrappedParams := []string{}
	// for _, p := range params {
	// 	wrappedParams = append(wrappedParams, `"`+p+`"`)
	// }
	// formattedParams := strings.Join(wrappedParams, ", ")

	// isRangeInt := 0
	// if isRange {
	// 	isRangeInt = 1
	// }

	// cmd := fmt.Sprintf(`call govim#defineFunction("%s", [%s], %d)`, name, formattedParams, isRangeInt)
	// err = n.Nvim.Command(cmd)

	// if err != nil {
	// 	return err
	// }
}

// DefineCommand defines the named command in Vim. name must begin with a
// capital letter. attrs is a series of attributes for the command; see :help
// E174 in Vim for more details.
func (n *NeoGovim) DefineCommand(name string, f VimCommandFunction, attrs ...CommAttr) error {
	var err error
	if name == "" {
		return fmt.Errorf("command name must not be empty")
	}
	r, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsUpper(r) {
		return fmt.Errorf("command name %q must begin with a capital letter", name)
	}

	funcHandle := commHandlePref + name

	wrappedF := func(nv *nvim.Nvim, flagsJSON string, args ...string) error {
		var flagVals CommandFlags
		err := flagVals.UnmarshalJSON([]byte(flagsJSON))
		if err != nil {
			return err
		}
		return f(n, flagVals, args...)
	}

	err = n.Nvim.RegisterHandler(funcHandle, wrappedF)
	if err != nil {
		return err
	}

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
			attrs = append(attrs, `"`+k.String()+`"`)
		}
		sort.Strings(attrs)
		attrMap["general"] = attrs
	}
	wrappedAttrs := []string{}
	for k, v := range attrMap {
		wrappedAttrs = append(wrappedAttrs, fmt.Sprintf(`"%s": %s`, k, v))
	}
	formattedAttrMap := strings.Join(wrappedAttrs, ", ")

	cmd := fmt.Sprintf(`call govim#defineCommand("%s", {%s})`, name, formattedAttrMap)
	err = n.Nvim.Command(cmd)

	if err != nil {
		return err
	}
	return nil
}

// DefineAutoCommand defines an autocmd for events for files matching patterns.
func (n *NeoGovim) DefineAutoCommand(group string, events Events, patts Patterns, nested bool, f VimAutoCommandFunction, exprs ...string) error {
	n.autocmdLock.Lock()
	funcHandle := fmt.Sprintf("%v%v", autoCommHandlePref, n.autocmdNextID)
	n.autocmdNextID++
	n.autocmdLock.Unlock()

	wrappedF := func(nv *nvim.Nvim, args []interface{}) error {
		jsonArgs := []json.RawMessage{}
		for _, a := range args {
			v, err := json.Marshal(a)
			if err != nil {
				return err
			}
			jsonArgs = append(jsonArgs, v)
		}
		return f(n, jsonArgs...)
	}

	err := n.Nvim.RegisterHandler(funcHandle, wrappedF)
	if err != nil {
		return err
	}

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

	if exprs == nil {
		// must be non-nil
		exprs = []string{}
	}
	wrappedExprs := []string{}
	for _, e := range exprs {
		wrappedExprs = append(wrappedExprs, `"`+e+`"`)
	}
	expr := fmt.Sprintf(`call govim#defineAutoCommand("%s","%s",[%s])`, funcHandle, def.String(), strings.Join(wrappedExprs, ", "))

	err = n.Nvim.Command(expr)

	if err != nil {
		return err
	}

	return nil
}

// Run is a user-friendly run wrapper
func (n *NeoGovim) Run() error {
	err := n.DoProto(func() error {
		n.run()
		return nil
	})
	n.tomb.Kill(err)
	var shutdownErr error
	if n.plugin != nil {
		shutdownErr = n.plugin.Shutdown()
		close(n.shutdown)
	}
	if n.pluginErrCh != nil {
		close(n.pluginErrCh)
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	if err := n.tomb.Wait(); err != nil {
		return err
	}
	return err
}

// DoProto is used as a wrapper around function calls that jump the "interface"
// between the user and protocol aspects of govim.
func (n *NeoGovim) DoProto(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r := r.(type) {
			case errProto:
				if r.underlying == io.EOF {
					return
				}
				err = r
			case error:
				if r == tomb.ErrDying {
					panic(ErrShuttingDown)
				}
				if r == ErrShuttingDown {
					panic(r)
				}
				err = r
			default:
				panic(r)
			}
		}
	}()
	err = f()
	return
}

// Viewport returns the active Vim viewport
func (n *NeoGovim) Viewport() (Viewport, error) {
	panic("Viewport() not implemented yet")
}

// Errorf raises a formatted fatal error
func (n *NeoGovim) Errorf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args)
	n.tomb.Kill(fmt.Errorf(format, args...))
}

// Logf logs a formatted message to the logger
func (n *NeoGovim) Logf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	if s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	t := time.Now().Format("2006-01-02T15:04:05.000000")
	s = strings.Replace(s, "\n", "\n"+t+": ", -1)
	fmt.Fprint(n.log, t+": "+s+"\n")
}

// Scheduled returns the event queue Govim interface
func (n *NeoGovim) Scheduled() Govim {
	return n
}

// Schedule schdules f to run in the event queue
func (n *NeoGovim) Schedule(f func(Govim) error) chan struct{} {
	done := make(chan struct{})
	go func() {
		f(n)
		done <- struct{}{}
	}()
	return done
}

func (n *NeoGovim) Flavor() Flavor {
	return FlavorNeovim
}

func (n *NeoGovim) Version() string {
	return n.version
}

func (n *NeoGovim) Loaded() chan struct{} {
	panic("Loaded() not implemented yet")
}

func (n *NeoGovim) Initialized() chan struct{} {
	return n.initialized
}

func (n *NeoGovim) Shutdown() chan struct{} {
	return n.shutdown
}

func (n *NeoGovim) unscheduledCallCallback(typ string, vs ...interface{}) unscheduledCallback {
	ch := make(unscheduledCallback)

	n.tombgo(func() error {
		n.callVim(ch, typ, vs...)
		return nil
	})
	return ch
}

// decodeJSON is a low-level protocol primitive for decoding a JSON value.
func (n *NeoGovim) decodeJSON(m json.RawMessage, i interface{}) {
	err := json.Unmarshal(m, i)
	if err != nil {
		n.errProto("failed to decode JSON into type %T: %v", i, err)
	}
}

func (n *NeoGovim) errProto(format string, args ...interface{}) {
	panic(errProto{
		underlying: fmt.Errorf(format, args...),
	})
}

// parseJSONArgSlice is a low-level protocol primitive for parsing a slice of
// raw encoded JSON values
func (n *NeoGovim) parseJSONArgSlice(m json.RawMessage) []json.RawMessage {
	var i []json.RawMessage
	n.decodeJSON(m, &i)
	return i
}

// parseString is a low-level protocol primtive for parsing a string from a
// raw encoded JSON value
func (n *NeoGovim) parseString(m json.RawMessage) string {
	var s string
	n.decodeJSON(m, &s)
	return s
}

// parseInt is a low-level protocol primtive for parsing an int from a
// raw encoded JSON value
func (n *NeoGovim) parseInt(m json.RawMessage) int {
	var i int
	n.decodeJSON(m, &i)
	return i
}

func (g *NeoGovim) handleVimRequest(msg json.RawMessage) (interface{}, error) {
	g.logVimEventf("recvJSONMsg: %s\n", msg)
	args := g.parseJSONArgSlice(msg)
	typ := g.parseString(args[0])
	args = args[1:]

	g.logVimEventf("recvJSONMsg parsed: %s %v\n", typ, args)

	var resp interface{}

	switch typ {
	case "function":
		fname := g.parseString(args[0])
		fargs := args[1:]
		fname, f := g.funcHandler(fname)
		var line1, line2 int
		// var call func() (interface{}, error)

		var err error
		switch f := f.(type) {
		case internalFunction:
			fargs = g.parseJSONArgSlice(fargs[0])
			// call = func() (interface{}, error) {
			resp, err = f(fargs...)
			// }
		case VimRangeFunction:
			line1 = g.parseInt(fargs[0])
			line2 = g.parseInt(fargs[1])
			fargs = g.parseJSONArgSlice(fargs[2])
			// call = func() (interface{}, error) {
			resp, err = f(nil, line1, line2, fargs...)
			// }
		case VimFunction:
			fargs = g.parseJSONArgSlice(fargs[0])
			// call = func() (interface{}, error) {
			resp, err = f(nil, fargs...)
			// }
		case VimCommandFunction:
			var flagVals CommandFlags
			g.decodeJSON(fargs[0], &flagVals)
			var args []string
			for _, f := range fargs[1:] {
				args = append(args, g.parseString(f))
			}
			// call = func() (interface{}, error) {
			err = f(nil, flagVals, args...)
			// }
		case VimAutoCommandFunction:
			fargs = g.parseJSONArgSlice(fargs[0])
			// call = func() (interface{}, error) {
			err = f(g, fargs...)
			// }
		default:
			g.Errorf("unknown function type for %v %T", fname, f)
		}
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func (n *NeoGovim) goHandleShutdown(f func() error) {
	n.tombgo(func() error {
		defer func() {
			if r := recover(); r != nil && r != ErrShuttingDown && r != tomb.ErrDying {
				panic(r)
			}
		}()
		return f()
	})
}

func (n *NeoGovim) runEventQueue() error {
	q := n.eventQueue
GetWork:
	for {
		work, wait := q.Get()
		if wait != nil {
			select {
			case <-n.tomb.Dying():
				return tomb.ErrDying
			case <-wait:
			}
			continue GetWork
		}
		n.goHandleShutdown(func() error {
			return work()
		})
		select {
		case <-n.tomb.Dying():
			return tomb.ErrDying
		case <-n.flushEvents:
		}
	}
}

func (n *NeoGovim) tombgo(f func() error) {
	n.tomb.Go(func() error {
		err := f()
		return err
	})
}

func (n *NeoGovim) handleVimShutdown() {
	n.Logf("Neovim shutting down - killing all tomb processes")
	n.shutdown <- struct{}{}
}

func (n *NeoGovim) logVimEventf(format string, args ...interface{}) {
	n.Logf("vim start =======================\n"+format+"vim end =======================\n", args...)
}

func (n *NeoGovim) callVim(ch callback, typ string, vs ...interface{}) error {
	// g.callbackRespsLock.Lock()
	// id := g.callVimNextID
	// g.callVimNextID++
	// g.callbackResps[id] = ch
	// g.callbackRespsLock.Unlock()
	args := []interface{}{99, typ}
	args = append(args, vs...)
	n.tombgo(func() error {
		switch ch := ch.(type) {
		case scheduledCallback:
			ch <- n.sendJSONMsg(0, args)
		case unscheduledCallback:
			ch <- n.sendJSONMsg(0, args)
		default:
			return fmt.Errorf("unknown type of callback responser: %T", ch)
		}
		return nil
	})
	return nil
}

// sendJSONMsg is a low-level protocol primitive for sending a JSON msg that will be
// understood by Vim. See https://vimhelp.org/channel.txt.html#channel-use
func (n *NeoGovim) sendJSONMsg(vimChannel, p1 interface{}, params ...interface{}) callbackResp {
	msg := []interface{}{vimChannel, p1}
	msg = append(msg, params...)
	// TODO could use a multi-writer here
	logMsg, err := json.Marshal(msg)
	if err != nil {
		// g.errProto("failed to create log message: %v", err)
		n.Errorf("failed to create log message: %v", err)
	}
	n.logVimEventf("sendJSONMsg: %s\n", logMsg)

	jsonArgs, err := json.Marshal(msg)
	if err != nil {
		n.Errorf("failed to marshal msg to JSON: %v", err)
	}
	expr := fmt.Sprintf(`govim#define('%s')`, string(jsonArgs))

	type callbackResponse struct {
		errString string
		args      []json.RawMessage
	}

	// type callbackResponseMsg struct {
	// 	typ       string
	// 	channelID int
	// 	response  callbackResponse
	// }

	var res []interface{}
	err = n.Nvim.Eval(expr, &res)
	if err != nil {
		n.logVimEventf("recvJSONMsg error: %v\n", err)
		return callbackResp{}
	}

	n.logVimEventf("recvJSONMsg: %v\n", res)

	if len(res) != 3 {
		n.logVimEventf("expected JSON array with 3 elements, got: %v\n", res)
		return callbackResp{}
	}

	_, ok := res[0].(string)
	if !ok {
		n.logVimEventf("couldn't marshal '%v' to a string\n", res[0])
		return callbackResp{}
	}
	// _, ok = res[1].(string)
	// if !ok {
	// 	n.logVimEventf("couldn't marshal '%v' to an int\n", res[1])
	// 	return callbackResp{}
	// }
	vals, ok := res[2].([]interface{})
	if !ok {
		n.logVimEventf("couldn't marshal '%v' to []interface{}\n", res[2])
		return callbackResp{}
	}

	var errString string
	var val json.RawMessage
	if len(vals) == 1 {
		errString, ok = vals[0].(string)
		if !ok {
			n.logVimEventf("couldn't marshal '%v' to string\n", vals[0])
			return callbackResp{}
		}
	}

	return callbackResp{
		errString: errString,
		val:       val,
	}
}

func (n *NeoGovim) handleChannelError(ch unscheduledCallback, err error, format string, args ...interface{}) error {
	_, err = n.handleChannelValueAndError(ch, err, format, args)
	return err
}

func (n *NeoGovim) handleChannelValueAndError(ch unscheduledCallback, err error, format string, args ...interface{}) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	args = append([]interface{}{}, args...)
	select {
	case <-n.tomb.Dying():
		panic(ErrShuttingDown)
	case resp := <-ch:
		if resp.errString != "" {
			args = append(args, resp.errString)
			return nil, fmt.Errorf(format, args...)
		}
		return resp.val, nil
	}
}

// funcHandler returns the
func (n *NeoGovim) funcHandler(name string) (string, interface{}) {
	n.funcHandlersLock.Lock()
	defer n.funcHandlersLock.Unlock()
	f, ok := n.funcHandlers[name]
	if !ok {
		n.errProto("tried to invoke %v but no function defined", name)
	}
	return strings.TrimPrefix(name, funcHandlePref), f
}
