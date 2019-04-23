package config

const (
	GlobalPrefix = "g:govim_"

	// GlobalFormatOnSave is a string value variable that configures which tool
	// to use for formatting on save.  Options are given by constants of type
	// FormatOnSave
	GlobalFormatOnSave = GlobalPrefix + "format_on_save"
)

type Command string

const (
	// CommandGoToDef jumps to the definition of the identifier under the cursor,
	// pushing the current location onto the jump stack. CommandGoToDef respects
	// &switchbuf
	CommandGoToDef Command = "GoToDef"

	// CommandGoToPrevDef jumps to the previous location in the jump stack.
	// CommandGoToPrevDef respects &switchbuf
	CommandGoToPrevDef Command = "GoToPrevDef"

	// CommandHello is a friendly command, largely for checking govim is
	// working.
	CommandHello Command = "Hello"
)

type Function string

const (
	// FunctionBalloonExpr is not intended to be called by the user. Instead it
	// is automatically set as the value of balloonexpr by govim.
	FunctionBalloonExpr Function = "BalloonExpr"

	// FunctionComplete is not intended to be called by the user. Instead it is
	// automatically set as the value of omnifunc by govim.
	FunctionComplete Function = "Complete"

	// FunctionHover returns the same text that would be returned by a
	// mouse-based hover, but instead uses the cursor position for the
	// identifier.
	FunctionHover Function = "Hover"

	// FunctionHello is a friendly function, largely for checking govim is
	// working.
	FunctionHello Function = "Hello"
)

// FormatOnSave typed constants define the set of valid values that
// GlobalFormatOnSave can take
type FormatOnSave string

const (
	// FormatOnSaveNone specifies that nothing should be done when a .go file is
	// saved
	FormatOnSaveNone FormatOnSave = ""

	// FormatOnSaveGoFmt specifies that gopls should run a gofmt-based
	// formatting on a .go file before as it is saved.
	FormatOnSaveGoFmt FormatOnSave = "gofmt"

	// FormatOnSaveGoImports specifies that gopls should run a goimports-based
	// formatting on a .go file before as it is saved.
	FormatOnSaveGoImports FormatOnSave = "goimports"
)
