package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/govim/govim"
)

type Driver struct {
	govim.Govim
	prefix string
}

type (
	// VimAutoCommandFunction is the signature of a callback from a defined autocmd
	DriverAutoCommandFunction func(args ...json.RawMessage) error

	// VimCommandFunction is the signature of a callback from a defined command
	DriverCommandFunction func(flags govim.CommandFlags, args ...string) error

	// VimFunction is the signature of a callback from a defined function
	DriverFunction func(args ...json.RawMessage) (interface{}, error)

	// DriverRangeFunction is the signature of a callback from a defined range-based
	// function
	DriverRangeFunction func(line1, line2 int, args ...json.RawMessage) (interface{}, error)
)

func NewDriver(name string) Driver {
	return Driver{
		prefix: name,
	}
}

func (d Driver) Prefix() string {
	return d.prefix
}

func (d Driver) clone(g govim.Govim) Driver {
	d.Govim = g
	return d
}

func (d Driver) do(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r := r.(type) {
			case ErrDriver:
				err = r
			default:
				panic(r)
			}
		}
	}()
	return f()
}

func (d Driver) errorf(check error, format string, args ...interface{}) {
	if check == govim.ErrShuttingDown {
		panic(check)
	}
	panic(ErrDriver{Underlying: fmt.Errorf(format, args...)})
}

func (d Driver) ChannelExpr(expr string) json.RawMessage {
	i, err := d.Govim.ChannelExpr(expr)
	if err != nil {
		d.errorf(err, "ChannelExpr(%q) failed: %v", expr, err)
	}
	return i
}

func (d Driver) ChannelCall(name string, args ...interface{}) json.RawMessage {
	i, err := d.Govim.ChannelCall(name, args...)
	if err != nil {
		d.errorf(err, "ChannelCall(%q) failed: %v", name, err)
	}
	return i
}

func (d Driver) ChannelEx(expr string) {
	if err := d.Govim.ChannelEx(expr); err != nil {
		d.errorf(err, "ChannelEx(%q) failed: %v", expr, err)
	}
}

func (d Driver) ChannelNormal(expr string) {
	if err := d.Govim.ChannelNormal(expr); err != nil {
		d.errorf(err, "ChannelNormal(%q) failed: %v", expr, err)
	}
}

func (d Driver) ChannelRedraw(force bool) {
	if err := d.Govim.ChannelRedraw(force); err != nil {
		d.errorf(err, "ChannelRedraw(%v) failed: %v", force, err)
	}
}

func (d Driver) Parse(j json.RawMessage, i interface{}) {
	if err := json.Unmarshal(j, i); err != nil {
		d.errorf(err, "failed to parse from %q: %v", j, err)
	}
}

func (d Driver) ParseString(j json.RawMessage) string {
	var v string
	if err := json.Unmarshal(j, &v); err != nil {
		d.errorf(err, "failed to parse string from %q: %v", j, err)
	}
	return v
}

func (d Driver) ParseJSONArgSlice(j json.RawMessage) []json.RawMessage {
	var v []json.RawMessage
	if err := json.Unmarshal(j, &v); err != nil {
		d.errorf(err, "failed to parse []json.RawMessage from %q: %v", j, err)
	}
	return v
}

func (d Driver) ParseInt(j json.RawMessage) int {
	var v int
	if err := json.Unmarshal(j, &v); err != nil {
		d.errorf(err, "failed to parse int from %q: %v", j, err)
	}
	return v
}

func (d Driver) ParseUint(j json.RawMessage) uint {
	var v uint
	if err := json.Unmarshal(j, &v); err != nil {
		d.errorf(err, "failed to parse int from %q: %v", j, err)
	}
	return v
}

func (d Driver) ChannelExprf(format string, args ...interface{}) json.RawMessage {
	return d.ChannelExpr(fmt.Sprintf(format, args...))
}

func (d Driver) ChannelExf(format string, args ...interface{}) {
	d.ChannelEx(fmt.Sprintf(format, args...))
}

func (d Driver) DefineFunction(name string, args []string, f DriverFunction) {
	if err := d.Govim.DefineFunction(d.prefix+name, args, d.doFunction(f)); err != nil {
		d.errorf(err, "failed to DefineFunction %q: %v", name, err)
	}
}

func (d Driver) DefineRangeFunction(name string, args []string, f DriverRangeFunction) {
	if err := d.Govim.DefineRangeFunction(d.prefix+name, args, d.doRangeFunction(f)); err != nil {
		d.errorf(err, "failed to DefineRangeFunction %q: %v", name, err)
	}
}

func (d Driver) DefineCommand(name string, f DriverCommandFunction, attrs ...govim.CommAttr) {
	if err := d.Govim.DefineCommand(d.prefix+name, d.doCommandFunction(f), attrs...); err != nil {
		d.errorf(err, "failed to DefineCommand %q: %v", name, err)
	}
}

func (d Driver) DefineAutoCommand(group string, events govim.Events, patts govim.Patterns, nested bool, f DriverAutoCommandFunction, exprs ...string) {
	if group == "" {
		group = strings.ToLower(d.prefix)
	}
	if err := d.Govim.DefineAutoCommand(group, events, patts, nested, d.doAutoCommandFunction(f), exprs...); err != nil {
		d.errorf(err, "failed to DefineAutoCommand: %v", err)
	}
}

func (d Driver) Viewport() govim.Viewport {
	vp, err := d.Govim.Viewport()
	if err != nil {
		d.errorf(err, "failed to get Viewport: %v", err)
	}
	return vp
}

func (d Driver) doFunction(f DriverFunction) govim.VimFunction {
	return func(g govim.Govim, args ...json.RawMessage) (interface{}, error) {
		d := d.clone(g)
		var i interface{}
		err := d.do(func() error {
			var err error
			i, err = f(args...)
			return err
		})
		if err != nil {
			return nil, err
		}
		return i, nil
	}
}

func (d Driver) doRangeFunction(f DriverRangeFunction) govim.VimRangeFunction {
	return func(g govim.Govim, first, last int, args ...json.RawMessage) (interface{}, error) {
		d := d.clone(g)
		var i interface{}
		err := d.do(func() error {
			var err error
			i, err = f(first, last, args...)
			return err
		})
		if err != nil {
			return nil, err
		}
		return i, nil
	}
}

func (d Driver) doCommandFunction(f DriverCommandFunction) govim.VimCommandFunction {
	return func(g govim.Govim, flags govim.CommandFlags, args ...string) error {
		d := d.clone(g)
		return d.do(func() error {
			return f(flags, args...)
		})
	}
}

func (d Driver) doAutoCommandFunction(f DriverAutoCommandFunction) govim.VimAutoCommandFunction {
	return func(g govim.Govim, args ...json.RawMessage) error {
		d := d.clone(g)
		return d.do(func() error {
			return f(args...)
		})
	}
}

type ErrDriver struct {
	Underlying error
}

func (e ErrDriver) Error() string {
	return fmt.Sprintf("driver error: %v", e.Underlying)
}
