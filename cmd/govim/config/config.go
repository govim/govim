// Package config declares the configuration variables, functions and commands
// used by govim
package config

const (
	InternalFunctionPrefix = "_internal_"
)

type Config struct {
	// FormatOnSave is a string value that configures which tool to use for
	// formatting on save. Options are given by constants of type FormatOnSave.
	// Default: FormatOnSaveGoImports.
	FormatOnSave FormatOnSave

	// QuickfixAutoDiagnosticsDisable is a boolean (0 or 1 in VimScript) that
	// controls whether auto-population of the quickfix window with gopls
	// diagnostics is disabled or not. When not disabled, govim waits for
	// updatetime (help updatetime) before populating the quickfix window with
	// the current gopls diagnostics. When disabled, the
	// CommandQuickfixDiagnostics command can be used to manually trigger the
	// population. Default: false (0)
	QuickfixAutoDiagnosticsDisable bool

	// QuickfixSignsDisable is a boolean (0 or 1 in VimScript) that controls
	// whether quickfix entries should be annotated with signs in the gutter.
	// Signs are placed when gopls diagnostics updates the quickfix list, either
	// automatically when QuickfixAutoDiagnosticsDisable is false, or when the
	// user run :GOVIMQuickfixDiagnostics.
	// Default: false (0)
	QuickfixSignsDisable bool

	// ExperimentalMouseTriggeredHoverPopupOptions is a map of options to apply
	// when creating hover-based popup windows triggered by the mouse hovering
	// over an identifier. It corresponds to the second argument to popup_create
	// (see :help popup_create-arguments). If set, these options define the
	// options that will be used when creating hover popups. That is to say, the
	// options set in ExperimentalMouseTriggeredHoverPopupOptions do no override
	// defaults set in govim, the supplied map is used as is in the call to
	// popup create. The only exceptions to this are the values of line and col.
	// Because the position of the popup is relative to positin of where it was
	// triggered, these values are interpreted as relative values. Hence a "line"
	// value of -1 will mean the popup is placed on the line before the position
	// at which the hover was triggered.
	//
	// The "filter" and "callback" options are not supported because they have a
	// function-type value, and hence cannot be serialised to JSON.
	//
	// This is an experimental feature designed to help iterate on
	// the most sensible out-of-the-box defaults for hover popups. It might go
	// away in the future, be renamed etc.
	ExperimentalMouseTriggeredHoverPopupOptions map[string]interface{}

	// ExperimentalCursorTriggeredHoverPopupOptions is a map of options to apply
	// when creating hover-based popup windows triggered by a call to
	// GOVIMHover() which uses the cursor position for popup placement.  It
	// corresponds to the second argument to popup_create (see :help
	// popup_create-arguments). If set, these options define the options that
	// will be used when creating hover popups. That is to say, the options set
	// in ExperimentalCursorTriggeredHoverPopupOptions do no override defaults
	// set in govim, the supplied map is used as is in the call to popup create.
	// The only exceptions to this are the values of line and col.  Because the
	// position of the popup is relative to positin of where it was triggered,
	// these values are interpreted as relative values. Hence a "line" value of
	// -1 will mean the popup is placed on the line before the position at which
	// the hover was triggered.
	//
	// The "filter" and "callback" options are not supported because they have a
	// function-type value, and hence cannot be serialised to JSON.
	//
	// This is an experimental feature designed to help iterate on the most
	// sensible out-of-the-box defaults for hover popups. It might go away in
	// the future, be renamed etc.
	ExperimentalCursorTriggeredHoverPopupOptions map[string]interface{}
}

type Command string

const (
	// CommandGoToDef jumps to the definition of the identifier under the cursor,
	// pushing the current location onto the jump stack. CommandGoToDef respects
	// &switchbuf
	CommandGoToDef Command = "GoToDef"

	// CommandGoToPrevDef jumps to the previous location in the jump stack.
	// CommandGoToPrevDef respects &switchbuf
	CommandGoToPrevDef Command = "GoToPrevDef"

	// CommandGoFmt applies gofmt to the entire buffer
	CommandGoFmt Command = "GoFmt"

	// CommandGoImports applies goimports to the entire buffer
	CommandGoImports Command = "GoImports"

	// CommandQuickfixDiagnostics populates the quickfix window with the current
	// gopls-reported diagnostics
	CommandQuickfixDiagnostics Command = "QuickfixDiagnostics"

	// CommandReferences finds references to the identifier under the cursor.
	CommandReferences Command = "References"

	// CommandRename renames the identifier under the cursor. If provided with an
	// argument, that argument is used as the new name. If not, the user is
	// prompted for the new identifier name.
	CommandRename Command = "Rename"

	// CommandStringFn applies a transformation function to text. Without a
	// range the current line is used as input. Visual ranges can also be used,
	// with the exception of visual blocks. The command takes one or more
	// arguments: the transformation functions to apply. Tab completion can be
	// used to complete against the defined transformation functions.
	//
	// The goal with this command is to expose standard library functions to
	// help manipulate text. Wherever possible, functions will directly map to
	// their standard library equivalents, for example, strconv.Quote. In this
	// case, the format is $importpath.$function. In some situations, poetic
	// license may be required.
	CommandStringFn Command = "StringFn"
)

type Function string

const (
	// FunctionBalloonExpr is an internal function used by govim for balloonexpr
	// in Vim
	FunctionBalloonExpr Function = InternalFunctionPrefix + "BalloonExpr"

	// FunctionComplete is an internal function used by govim as for omnifunc in
	// Vim
	FunctionComplete Function = InternalFunctionPrefix + "Complete"

	// FunctionHover returns the same text that would be returned by a
	// mouse-based hover, but instead uses the cursor position for the
	// identifier.
	FunctionHover Function = "Hover"

	// FunctionBufChanged is an internal function used by govim for handling
	// delta-based changes in buffers.
	FunctionBufChanged Function = InternalFunctionPrefix + "BufChanged"

	// FunctionEnrichDelta is an internal function used by govim for enriching
	// listener_add based callbacks before calling FunctionBufChanged
	FunctionEnrichDelta Function = InternalFunctionPrefix + "EnrichDelta"

	// FunctionSetConfig is an internal function used by govim for pushing config
	// changes from Vim to govim.
	FunctionSetConfig Function = InternalFunctionPrefix + "SetConfig"

	// FunctionSetUserBusy is an internal function used by govim for indicated
	// whether the user is busy or not (based on cursor movement)
	FunctionSetUserBusy Function = InternalFunctionPrefix + "SetUserBusy"

	// FunctionStringFnComplete is an internal function used by govim to provide
	// completion of arguments to CommandStringFn
	FunctionStringFnComplete Function = InternalFunctionPrefix + "StringFnComplete"

	// FunctionMotion moves the cursor according to the arguments provided.
	FunctionMotion Function = "Motion"
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
