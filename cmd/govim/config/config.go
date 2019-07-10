// Package config declares the configuration variables, functions and commands
// used by govim
package config

const (
	internalFunctionPrefix = "_internal_"
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

	// CommandHello is a friendly command, largely for checking govim is
	// working.
	CommandHello Command = "Hello"

	// CommandGoFmt applies gofmt to the entire buffer
	CommandGoFmt Command = "GoFmt"

	// CommandGoImports applies goimports to the entire buffer
	CommandGoImports Command = "GoImports"

	// CommandQuickfixDiagnostics populates the quickfix window with the current
	// gopls-reported diagnostics
	CommandQuickfixDiagnostics Command = "QuickfixDiagnostics"

	// CommandReferences finds references to the identifier under the cursor.
	CommandReferences = "References"
)

type Function string

const (
	// FunctionBalloonExpr is an internal function used by govim for balloonexpr
	// in Vim
	FunctionBalloonExpr Function = internalFunctionPrefix + "BalloonExpr"

	// FunctionComplete is an internal function used by govim as for omnifunc in
	// Vim
	FunctionComplete Function = internalFunctionPrefix + "Complete"

	// FunctionHover returns the same text that would be returned by a
	// mouse-based hover, but instead uses the cursor position for the
	// identifier.
	FunctionHover Function = "Hover"

	// FunctionHello is a friendly function, largely for checking govim is
	// working.
	FunctionHello Function = "Hello"

	// FunctionBufChanged is an internal function used by govim for handling
	// delta-based changes in buffers.
	FunctionBufChanged = internalFunctionPrefix + "BufChanged"

	// FunctionEnrichDelta is an internal function used by govim for enriching
	// listener_add based callbacks before calling FunctionBufChanged
	FunctionEnrichDelta = internalFunctionPrefix + "EnrichDelta"

	// FunctionSetConfig is an internal function used by govim for pushing config
	// changes from Vim to govim.
	FunctionSetConfig = internalFunctionPrefix + "SetConfig"

	// FunctionSetUserBusy is an internal function used by govim for indicated
	// whether the user is busy or not (based on cursor movement)
	FunctionSetUserBusy = internalFunctionPrefix + "SetUserBusy"

	// FunctionDumpPopups is an internal function used by govim tests for capturing
	// the text in all visible popups
	FunctionDumpPopups = internalFunctionPrefix + "DumpPopups"
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
