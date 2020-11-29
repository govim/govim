package types

import "strings"

// PopupLine is the internal representation of a single text line with text
// propertiesin a vim popup. When creating popups using popup_create, the
// first arg can be either a buffer number, a string, a list of strings or
// a list of text lines with text properties.
type PopupLine struct {
	Text  string      `json:"text"`
	Props []PopupProp `json:"props"`
}

// PopupProp is the internal representation of a single text property used
// in a popup line. It describes where on that line the property begin
// (where Col is 1-indexed) and the length. Type must be an existing
// text property type (defined by calling prop_type_add in vim).
type PopupProp struct {
	Type string `json:"type"`
	Col  int    `json:"col"`
	Len  int    `json:"length"`
}

type ProgressInitiator string

const (
	GoTest                 ProgressInitiator = "GoTest"
	WorkDoneProgressCreate ProgressInitiator = "WorkDoneProgressCreate"
)

// ProgressPopup represents a vim popup placed in the upper right corner used
// to show LSP progress. LinePos is used to stack multiple visible progress
// popups. Initiator is a optional field used to describe who initiated this
// progress (if known), e.g. "GoTest" when running GOVIMGoTest. This allow
// us to handle text from different commands to be handled differently (or
// even suppressed).
type ProgressPopup struct {
	ID        int
	Text      strings.Builder
	LinePos   int
	Initiator ProgressInitiator
}
