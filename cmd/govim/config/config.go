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
