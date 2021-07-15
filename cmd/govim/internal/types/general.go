package types

import (
	"github.com/govim/govim/cmd/govim/config"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

// CursorPosition represents a cursor position within a window
type CursorPosition struct {
	*Point

	BufNr         int
	WinNr         int
	WinID         int
	ScreenRow     int
	ScreenCol     int
	ScreenEndCol  int
	ScreenCursCol int
}

// A WatchedFile is a file we are watching but that is not loaded as a buffer
// in Vim
type WatchedFile struct {
	Path     string
	Version  int
	Contents []byte
}

func (w *WatchedFile) URI() span.URI {
	return span.URIFromPath(w.Path)
}

// Range represents a range within a Buffer. Create ranges using NewRange
type Range struct {
	Start Point
	End   Point
}

// Diagnostic is the govim internal representation of a LSP diagnostic, used to
// populate quickfix list, place signs, highlight text ranges etc.
type Diagnostic struct {
	Filename string
	Source   string
	Range    Range
	Text     string
	Buf      int
	Severity Severity
}

// Severity is the govim internal representation of the LSP DiagnosticSeverites
type Severity int

const (
	SeverityErr  = Severity(protocol.SeverityError)
	SeverityWarn = Severity(protocol.SeverityWarning)
	SeverityInfo = Severity(protocol.SeverityInformation)
	SeverityHint = Severity(protocol.SeverityHint)
)

// SeverityPriority is used when placing signs and text property highlights.
// Values are based on the default value for signs, 10.
var SeverityPriority = map[Severity]int{
	SeverityErr:  14,
	SeverityWarn: 12,
	SeverityInfo: 10,
	SeverityHint: 8,
}

// SeverityHighlight returns corresponding highlight name for a severity.
var SeverityHighlight = map[Severity]config.Highlight{
	SeverityErr:  config.HighlightErr,
	SeverityWarn: config.HighlightWarn,
	SeverityInfo: config.HighlightInfo,
	SeverityHint: config.HighlightHint,
}

// SeverityHoverHighlight returns corresponding hover highlight name for a severity.
var SeverityHoverHighlight = map[Severity]config.Highlight{
	SeverityErr:  config.HighlightHoverErr,
	SeverityWarn: config.HighlightHoverWarn,
	SeverityInfo: config.HighlightHoverInfo,
	SeverityHint: config.HighlightHoverHint,
}

// TextPropID is the govim internal mapping of ID used when adding/removing text properties
type TextPropID int

const (
	DiagnosticTextPropID = 0
	ReferencesTextPropID = 1
	SemanticTokenPropID  = 2
)

// SemanticTokenHighlight returns corresponding highlight name for a semantic token.
// It also acts as a list of all semantic tokens supported by govim.
// TODO: We should use GOVIM defined highlights instead so that the user can alter the
// highlights.
var SemanticTokenHighlight = map[string]string{
	"namespace": "Operator",
	"type":      "Type",
	"parameter": "Identifier",
	"variable":  "Normal",
	"function":  "Function",
	"member":    "Function",
	"keyword":   "Keyword",
	"comment":   "Comment",
	"string":    "String",
	"number":    "Number",
	"operator":  "Operator",

	// TODO: There follwing commented ones wasn't used by gopls when semantic token
	// support was added. We should provide some sane highlight for them as well.
	//	"class":         "",
	//	"enum":          "",
	//	"interface":     "",
	//	"struct":        "",
	//	"typeParameter": "",
	//	"property":      "",
	//	"enumMember":    "",
	//	"event":         "",
	//	"macro":         "",
	//	"modifier":      "",
	//	"regexp":        "",
}
