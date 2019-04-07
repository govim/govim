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
	CommandGoToDef     Command = "GoToDef"
	CommandGoToPrevDef Command = "GoToPrevDef"
	CommandHello       Command = "Hello"
)

type Function string

const (
	FunctionBalloonExpr Function = "BalloonExpr"
	FunctionComplete    Function = "Complete"
	FunctionHello       Function = "Hello"
)

type FormatOnSave string

const (
	FormatOnSaveNone      FormatOnSave = ""
	FormatOnSaveGoFmt     FormatOnSave = "gofmt"
	FormatOnSaveGoImports FormatOnSave = "goimports"
)

type GoToDefMode string

const (
	GoToDefModeTab    GoToDefMode = "tab"
	GoToDefModeSplit  GoToDefMode = "split"
	GoToDefModeVsplit GoToDefMode = "vsplit"
)
