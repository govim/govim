package config

const (
	GlobalPrefix = "g:govim_"

	// GlobalFormatOnSave is a string value variable that configures which tool
	// to use for formatting on save.  Options are given by constants of type
	// FormatOnSave
	GlobalFormatOnSave = GlobalPrefix + "format_on_save"
)

type FormatOnSave string

const (
	FormatOnSaveGoFmt     FormatOnSave = "gofmt"
	FormatOnSaveGoImports FormatOnSave = "goimports"
)
