package govim

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type CommandFlags struct {
	Line1 *int
	Line2 *int
	Range *int
	Count *int
	Bang  *bool
	Reg   *string
}

func (c *CommandFlags) UnmarshalJSON(b []byte) error {
	var v struct {
		Line1 *int    `json:"line1"`
		Line2 *int    `json:"line2"`
		Range *int    `json:"range"`
		Count *int    `json:"count"`
		Bang  *string `json:"bang"`
		Reg   *string `json:"reg"`
	}

	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	c.Line1 = v.Line1
	c.Line2 = v.Line2
	c.Range = v.Range
	c.Count = v.Count
	if v.Bang != nil {
		b := *v.Bang == "!"
		c.Bang = &b
	}

	return nil
}

type CommAttr interface {
	fmt.Stringer
	isCommAttr()
}

type GenAttr uint

func (g GenAttr) isCommAttr() {}

//go:generate gobin -m -run golang.org/x/tools/cmd/stringer -type=GenAttr -linecomment -output gen_genattr_stringer.go

const (
	AttrBang     GenAttr = iota // -bang
	AttrBar                     // -bar
	AttrRegister                // -register
	AttrBuffer                  // -buffer
)

type Complete uint

func (c Complete) isCommAttr() {}

//go:generate gobin -m -run golang.org/x/tools/cmd/stringer -type=Complete -linecomment -output gen_complete_stringer.go

const (
	CompleteArglist      Complete = iota // -complete=arglist
	CompleteAugroup                      // -complete=augroup
	CompleteBuffer                       // -complete=buffer
	CompleteBehave                       // -complete=behave
	CompleteColor                        // -complete=color
	CompleteCommand                      // -complete=command
	CompleteCompiler                     // -complete=compiler
	CompleteCscope                       // -complete=cscope
	CompleteDir                          // -complete=dir
	CompleteEnvironment                  // -complete=environment
	CompleteEvent                        // -complete=event
	CompleteExpression                   // -complete=expression
	CompleteFile                         // -complete=file
	CompleteFileInPath                   // -complete=file_in_path
	CompleteFiletype                     // -complete=filetype
	CompleteFunction                     // -complete=function
	CompleteHelp                         // -complete=help
	CompleteHighlight                    // -complete=highlight
	CompleteHistory                      // -complete=history
	CompleteLocale                       // -complete=locale
	CompleteMapclear                     // -complete=mapclear
	CompleteMapping                      // -complete=mapping
	CompleteMenu                         // -complete=menu
	CompleteMessages                     // -complete=messages
	CompleteOption                       // -complete=option
	CompletePackadd                      // -complete=packadd
	CompleteShellCmd                     // -complete=shellcmd
	CompleteSign                         // -complete=sign
	CompleteSyntax                       // -complete=syntax
	CompleteSyntime                      // -complete=syntime
	CompleteTag                          // -complete=tag
	CompleteTagListFiles                 // -complete=tag_listfiles
	CompleteUser                         // -complete=user
	CompleteVar                          // -complete=var
)

type CompleteCustom string

func (c CompleteCustom) isCommAttr() {}

func (c CompleteCustom) String() string {
	return "-complete=custom," + string(c)
}

type CompleteCustomList string

func (c CompleteCustomList) isCommAttr() {}

func (c CompleteCustomList) String() string {
	return "-complete=customlist," + string(c)
}

type RangeN int

func (r RangeN) isCommAttr() {}

func (r RangeN) String() string {
	return strconv.Itoa(int(r))
}

type CountN int

func (c CountN) isCommAttr() {}

func (c CountN) String() string {
	return fmt.Sprintf("-count=%v", int(c))
}

type Range uint

func (r Range) isCommAttr() {}

//go:generate gobin -m -run golang.org/x/tools/cmd/stringer -type=Range -linecomment -output gen_range_stringer.go

const (
	RangeLine Range = iota // -range
	RangeFile              // -range=%
)

type NArgs uint

func (n NArgs) isCommAttr() {}

//go:generate gobin -m -run golang.org/x/tools/cmd/stringer -type=NArgs -linecomment -output gen_nargs_stringer.go

const (
	NArgs0          NArgs = iota // -nargs=0
	NArgs1                       // -nargs=1
	NArgsZeroOrMore              // -nargs=*
	NArgsZeroOrOne               // -nargs=?
	NArgsOneOrMore               // -nargs=+
)
