// Package config declares the configuration variables, functions and commands
// used by govim
package config

const (
	InternalFunctionPrefix = "_internal_"
)

type EnvVar string

const (
	// EnvVarUseGoplsFromPath is an environment variable which, when set to the
	// value "true", configures govim to use the gopls binary found in PATH
	// instead of the version required by the govim module.
	//
	// WARNING: use of this environment variable comes with no warranty, because
	// we have no guarantees that the gopls version found in PATH works with
	// govim.
	EnvVarUseGoplsFromPath EnvVar = "GOVIM_USE_GOPLS_FROM_PATH"

	// EnvVarGoplsFlags is an environment variable which, when set, will be used
	// to pass the value as flags to gopls.
	EnvVarGoplsFlags EnvVar = "GOVIM_GOPLS_FLAGS"

	// EnvVarGoplsVerbose is an environment variable which, when set to the
	// value "true", configures gopls' verboseOutput option.
	EnvVarGoplsVerbose EnvVar = "GOVIM_GOPLS_VERBOSE_OUTPUT"

	// EnvVarGoplsGOMAXPROCSMinusN is an environment variable which limits the
	// amount of CPU gopls can use. If an integer value n is supplied such that:
	// 0 < n < runtime.NumCPU(), gopls is run in an environment where
	// GOMAXPROCS=runtime.NumCPU() - n. If a percentage value p is supplied,
	// e.g. 20%, gopls is run in an environment where
	// GOMAXPROCS=math.Floor(runtime.NumCPUs() * (1-p)). govim will panic if supplied
	// with a value of n or p that would result in 0 >= GOMAXPROCS or
	// GOMAXPROCS > runtime.NumCPU()
	EnvVarGoplsGOMAXPROCSMinusN EnvVar = "GOVIM_GOPLS_GOMAXPROCS_MINUS_N"
)

//go:generate go run github.com/govim/govim/cmd/govim/config/internal/applygen Config

type Config struct {
	// FormatOnSave is a string value that configures which tool to use for
	// formatting on save. Options are given by constants of type FormatOnSave.
	//
	// Default: FormatOnSaveGoImportsGoFmt.
	FormatOnSave *FormatOnSave `json:",omitempty"`

	// QuickfixAutoDiagnostics is a boolean (0 or 1 in VimScript) that controls
	// whether auto-population of the quickfix window with gopls diagnostics is
	// enabled or not. When enabled, govim waits for updatetime (help
	// updatetime) before populating the quickfix window with the current gopls
	// diagnostics. When disabled, CommandQuickfixDiagnostics can be used to
	// manually trigger the population.
	//
	// Default: true
	QuickfixAutoDiagnostics *bool `json:",omitempty"`

	// QuickfixSigns is a boolean (0 or 1 in VimScript) that controls whether
	// diagnostic errors should be shown with signs in the gutter. When enabled,
	// govim waits for updatetime (help updatetime) before placing signs
	// using the current gopls diagnostics.
	//
	// Default: true
	QuickfixSigns *bool `json:",omitempty"`

	// HighlightDiagnostics enables in-code highlighting of diagnostics using
	// text properties. Each diagnostic reported by gopls will be highlighted
	// according to it's severity, using the following vim defined highlight
	// groups: GOVIMErr, GOVIMWarn, GOVIMInfo & GOVIMHint.
	//
	// Default: true
	HighlightDiagnostics *bool `json:",omitempty"`

	// HighlightReferences is a boolean (0 or 1 in VimScript) that controls
	// whether references to what is currently under the cursor should be
	// highlighted. When enabled, govim waits for updatetime (help updatetime)
	// before adding text properties covering each reference.
	//
	// Override the vim highlight group GOVIMReferences to alter the text
	// property style.
	//
	// Default: true
	HighlightReferences *bool `json:",omitempty"`

	// HoverDiagnostics is a boolean (0 or 1 in VimScript) that controls
	// whether diagnostics should be shown in the hover popup. When enabled
	// each diagnostic that covers the cursor/mouse position will be added
	// to the popup and formatted using text properties with the following
	// highlight groups: GOVIMHoverErr, GOVIMHoverWarn, GOVIMHoverInfo and
	// GOVIMHoverHint. The diagnostic source part is formatted via highlight
	// group GOVIMHoverDiagSrc.
	// All text properties are combined into existing syntax (with diagnostic
	// source being applied last) to provide a wide range of styles.
	//
	// Default: true
	HoverDiagnostics *bool `json:",omitempty"`

	// CompletionDeepCompletiions enables gopls' deep completion option
	// in the derivation of completion candidates.
	//
	// Default: true
	CompletionDeepCompletions *bool `json:",omitempty"`

	// CompletionMatcher is a string value that tells gopls which matcher
	// to use when computing completion candidates.
	//
	// Default: CompletionMatcherFuzzy
	CompletionMatcher *CompletionMatcher `json:",omitempty"`

	// Staticcheck enables staticcheck analyses in gopls
	//
	// Default: false
	Staticcheck *bool `json:",omitempty"`

	// CompleteUnimported configures gopls to attempt completions for unimported
	// standard library packages. e.g. when a user completes rand.<>, propose
	// rand.Seed (from math/rand) and rand.Prime (from crypto/rand), etc.
	//
	// Default: true
	CompleteUnimported *bool `json:",omitempty"`

	// GoImportsLocalPrefix is used to specify goimports's -local behavior. When
	// set, put imports beginning with this string after 3rd-party packages;
	// comma-separated list
	GoImportsLocalPrefix *string `json:",omitempty"`

	// CompletionBudget is the soft latency string-format time.Duration goal for
	// gopls completion requests. Most requests finish in a couple milliseconds,
	// but in some cases deep completions can take much longer. As we use up our
	// budget we dynamically reduce the search scope to ensure we return timely
	// results. Zero seconds means unlimited. Examples values: "0s", "100ms"
	CompletionBudget *string `json:",omitempty"`

	// TempModfile corresponds to the gopls config setting
	// "tempModfile" which controls whether a temporary modfile is used in place
	// of the main module's original go.mod file. When enabled, any
	// user-initiated changes (to .go files) that would otherwise have resulted
	// in changes to the original go.mod file, e.g. adding an import for a
	// package whose module is not listed as a requirement, get raised as
	// diagnostic warnings with suggested fixes which update the go.mod file.
	// Those diagnostic warnings are not, however, yet in place: see
	// https://go-review.googlesource.com/c/tools/+/216277.
	//
	// Default: false
	TempModfile *bool `json:",omitempty"`

	// GoplsEnv configures the set of environment variables gopls is using in
	// calls to go/packages. This is most easily understood in the context of
	// build tags/constraints where GOOS/GOARCH could be set, or by setting set
	// GOFLAGS=-modfile=go.local.mod in order to use an alternative go.mod file.
	GoplsEnv *map[string]string `json:",omitempty"`

	// ExperimentalAutoreadLoadedBuffers is used to reload buffers that are
	// changed outside vim even when they are loaded (e.g. running two vim
	// sessions in the same workspace). This is achieved by running "checktime"
	// when a file system event is handled. For this to work, vim must be
	// configured to hide buffers instead of abandon them. It is also
	// recommended to set autoread in vim to avoid a confirmation prompt when
	// the buffer isn't modified.
	// Recommended additions to vimrc:
	//
	// set hidden
	// set autoread
	//
	// Default: false
	ExperimentalAutoreadLoadedBuffers *bool `json:",omitempty"`

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
	//
	// Default: nil
	ExperimentalMouseTriggeredHoverPopupOptions *map[string]interface{} `json:",omitempty"`

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
	//
	// Default: nil
	ExperimentalCursorTriggeredHoverPopupOptions *map[string]interface{} `json:",omitempty"`

	// ExperimentalWorkaroundCompleteoptLongest provides a partial workaround
	// for users who would otherwise set completeopt+=longest but can't because
	// of github.com/vim/vim/issues/5891. That bug prevents completeopt+=longest
	// from working properly, and as the comments in that issue describe why the
	// workaround needs to be in govim. Set this config option along with
	// completeopt=menu,popup and Vim+govim will behave approximately like
	// completeopt+=longest.
	ExperimentalWorkaroundCompleteoptLongest *bool `json:",omitempty"`
}

type Command string

const (
	// CommandGoToDef jumps to the definition of the identifier under the cursor,
	// pushing the current location onto the jump stack. CommandGoToDef respects
	// &switchbuf
	CommandGoToDef Command = "GoToDef"

	// CommandGoToTypeDef jumps to the definition of the identifier under the
	// cursor, pushing the current location onto the jump stack.
	// CommandGoToTypeDef respects &switchbuf
	CommandGoToTypeDef Command = "GoToTypeDef"

	// CommandGoToPrevDef jumps to the previous location in the jump stack.
	// CommandGoToPrevDef respects &switchbuf
	CommandGoToPrevDef Command = "GoToPrevDef"

	// CommandGoFmt applies gofmt to the entire buffer
	CommandGoFmt Command = "GoFmt"

	// CommandGoImports fixes missing imports in the buffer much like the
	// old goimports command, but it does not format the buffer.
	CommandGoImports Command = "GoImports"

	// CommandQuickfixDiagnostics populates the quickfix window with the current
	// gopls-reported diagnostics
	CommandQuickfixDiagnostics Command = "QuickfixDiagnostics"

	// CommandReferences finds references to the identifier under the cursor.
	CommandReferences Command = "References"

	// CommandImplements finds all interfaces implemented by the type of the
	// identifier under the cursor.
	CommandImplements Command = "Implements"

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

	// CommandSuggestedFixes
	CommandSuggestedFixes Command = "SuggestedFixes"

	// CommandHighlightReferences highlights references to the identifier under
	// the cursor. The highlights are removed by a change to any file or a call
	// to CommandClearReferencesHighlights.
	CommandHighlightReferences Command = "HighlightReferences"

	// CommandClearReferencesHighlights clears any highlighint of references
	// added by a previous call to CommandHighlightReferences
	CommandClearReferencesHighlights Command = "ClearReferencesHighlights"

	// CommandExperimentalSignatureHelp shows a popup with signature information
	// and documentation for the command or method call enclosed by the cursor
	// position. The cursor must be after the left parentheses of the call
	// expression. If there is no signature help available for the current
	// cursor position, no popup is shown. Moving the cursor or the mouse causes
	// the popup to be dismissed.
	CommandExperimentalSignatureHelp Command = "ExperimentalSignatureHelp"
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

	FunctionPopupSelection Function = InternalFunctionPrefix + "PopupSelection"

	// FunctionStringFnComplete is an internal function used by govim to provide
	// completion of arguments to CommandStringFn
	FunctionStringFnComplete Function = InternalFunctionPrefix + "StringFnComplete"

	// FunctionMotion moves the cursor according to the arguments provided.
	FunctionMotion Function = "Motion"
)

// FormatOnSave typed constants define the set of valid values that
// Config.FormatOnSave can take
type FormatOnSave string

const (
	// FormatOnSaveNone specifies that nothing should be done when a .go file is
	// saved
	FormatOnSaveNone FormatOnSave = ""

	// FormatOnSaveGoFmt specifies that gopls should run CommandGoFmt formatting
	// on a .go file before it is saved
	FormatOnSaveGoFmt FormatOnSave = "gofmt"

	// FormatOnSaveGoImports specifies that gopls should run CommandGoImports
	// import fixing on a .go file before it is saved.
	FormatOnSaveGoImports FormatOnSave = "goimports"

	// FormatOnSaveGoImportsGoFmt specifies that gopls should run
	// CommandGoImports followed by CommandGoFmt on a .go file before it is
	// saved
	FormatOnSaveGoImportsGoFmt FormatOnSave = "goimports-gofmt"
)

// CompletionMatcher typed constants define the set of valid values that
// Config.Matcher can take
type CompletionMatcher string

const (
	// CompletionMatcherFuzzy specifies that gopls should use fuzzy matching
	// when computing completion candidates
	CompletionMatcherFuzzy CompletionMatcher = "fuzzy"

	// CompletionMatcherCaseSensitive specifies that gopls should use
	// case-sensitive matching when computing completion candidates
	CompletionMatcherCaseSensitive CompletionMatcher = "caseSensitive"

	// CompletionMatcherCaseInsensitive specifies that gopls should use
	// case-sensitive matching when computing completion candidates
	CompletionMatcherCaseInsensitive CompletionMatcher = "caseInsensitive"
)

// Highlight typed constants define the different highlight groups used by govim.
// All highlights can be overridden in vimrc, e.g.:
//
// highlight GOVIMErr ctermfg=16 ctermbg=4
type Highlight string

const (
	// HighlightErr is the group used to add text properties to errors
	HighlightErr Highlight = "GOVIMErr"
	// HighlightWarn is the group used to add text properties to warnings
	HighlightWarn Highlight = "GOVIMWarn"
	// HighlightInfo is the group used to add text properties to informations
	HighlightInfo Highlight = "GOVIMInfo"
	// HighlightHints is the group used to add text properties to hints
	HighlightHint Highlight = "GOVIMHint"

	// HighlightSignErr is the group used to add error signs in the gutter
	HighlightSignErr Highlight = "GOVIMSignErr"
	// HighlightSignWarn is the group used to add warning signs in the gutter
	HighlightSignWarn Highlight = "GOVIMSignWarn"
	// HighlightSignInfo is the group used to add info signs in the gutter
	HighlightSignInfo Highlight = "GOVIMSignInfo"
	// HighlightSignHint is the group used to add hint signs in the gutter
	HighlightSignHint Highlight = "GOVIMSignHint"

	// HighlightHoverErr is ths group used to add errors to the hover popup
	HighlightHoverErr Highlight = "GOVIMHoverErr"
	// HighlightHoverWarn is ths group used to add warnings to the hover popup
	HighlightHoverWarn Highlight = "GOVIMHoverWarn"
	// HighlightHoverInfo is ths group used to add informations to the hover popup
	HighlightHoverInfo Highlight = "GOVIMHoverInfo"
	// HighlightHoverHint is ths group used to add hints to the hover popup
	HighlightHoverHint Highlight = "GOVIMHoverHint"

	// HighlightHoverDiagSrc is the group used to format the source part of a hover diagnostic
	HighlightHoverDiagSrc Highlight = "GOVIMHoverDiagSrc"

	// HighlightReferences is the group used to add text properties to references
	HighlightReferences Highlight = "GOVIMReferences"
)
