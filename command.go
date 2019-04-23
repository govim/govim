package govim

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type CommandFlags struct {
	Line1 *int
	Line2 *int
	Range *int
	Count *int
	Bang  *bool
	Reg   *string
	Mods  CommModList
}

func (c *CommandFlags) UnmarshalJSON(b []byte) error {
	var v struct {
		Line1 *int    `json:"line1"`
		Line2 *int    `json:"line2"`
		Range *int    `json:"range"`
		Count *int    `json:"count"`
		Bang  *string `json:"bang"`
		Reg   *string `json:"reg"`
		Mods  string  `json:"mods"`
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
	for _, v := range strings.Fields(v.Mods) {
		cm := CommMod(v)
		switch cm {
		case CommModAboveLeft, CommModBelowRight, CommModBotRight, CommModBrowse, CommModConfirm,
			CommModHide, CommModKeepAlt, CommModKeepJumps, CommModKeepMarks, CommModKeepPatterns,
			CommModLeftAbove, CommModLockMarks, CommModNoSwapfile, CommModRightBelow, CommModSilent,
			CommModTab, CommModTopLeft, CommModVerbose, CommModVertical:
		default:
			return fmt.Errorf("unknown CommMod %q", cm)
		}
		c.Mods = append(c.Mods, cm)
	}

	return nil
}

type CommModList []CommMod

func (c CommModList) String() string {
	var vals []string
	for _, cc := range c {
		vals = append(vals, string(cc))
	}
	return strings.Join(vals, " ")
}

type CommMod string

const (
	CommModAboveLeft    CommMod = "aboveleft"
	CommModBelowRight   CommMod = "belowright"
	CommModBotRight     CommMod = "botright"
	CommModBrowse       CommMod = "browse"
	CommModConfirm      CommMod = "confirm"
	CommModHide         CommMod = "hide"
	CommModKeepAlt      CommMod = "keepalt"
	CommModKeepJumps    CommMod = "keepjumps"
	CommModKeepMarks    CommMod = "keepmarks"
	CommModKeepPatterns CommMod = "keeppatterns"
	CommModLeftAbove    CommMod = "leftabove"
	CommModLockMarks    CommMod = "lockmarks"
	CommModNoSwapfile   CommMod = "noswapfile"
	CommModRightBelow   CommMod = "rightbelow"
	CommModSilent       CommMod = "silent"
	CommModTab          CommMod = "tab"
	CommModTopLeft      CommMod = "topleft"
	CommModVerbose      CommMod = "verbose"
	CommModVertical     CommMod = "vertical"
)

type CommAttr interface {
	fmt.Stringer
	isCommAttr()
}

type GenAttr uint

func (g GenAttr) isCommAttr() {}

const (
	AttrBang     GenAttr = iota // -bang
	AttrBar                     // -bar
	AttrRegister                // -register
	AttrBuffer                  // -buffer
)

type Complete uint

func (c Complete) isCommAttr() {}

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

const (
	RangeLine Range = iota // -range
	RangeFile              // -range=%
)

type NArgs uint

func (n NArgs) isCommAttr() {}

const (
	NArgs0          NArgs = iota // -nargs=0
	NArgs1                       // -nargs=1
	NArgsZeroOrMore              // -nargs=*
	NArgsZeroOrOne               // -nargs=?
	NArgsOneOrMore               // -nargs=+
)
