package govim

type CompleteMode string

const (
	CompleteModeNone          CompleteMode = ""
	CompleteModeKeyword       CompleteMode = "keyword"
	CompleteModeCtrl_x        CompleteMode = "ctrl_x"
	CompleteModeWhole_line    CompleteMode = "whole_line"
	CompleteModeFiles         CompleteMode = "files"
	CompleteModeTags          CompleteMode = "tags"
	CompleteModePath_defines  CompleteMode = "path_defines"
	CompleteModePath_patterns CompleteMode = "path_patterns"
	CompleteModeDictionary    CompleteMode = "dictionary"
	CompleteModeThesaurus     CompleteMode = "thesaurus"
	CompleteModeCmdline       CompleteMode = "cmdline"
	CompleteModeFunction      CompleteMode = "function"
	CompleteModeOmni          CompleteMode = "omni"
	CompleteModeSpell         CompleteMode = "spell"
	CompleteModeEval          CompleteMode = "eval"
	CompleteModeUnknown       CompleteMode = "unknown"
)

type CompleteItem struct {
	Abbr     string `json:"abbr"`
	Word     string `json:"word"`
	Info     string `json:"info"`
	Menu     string `json:"menu"`
	UserData string `json:"user_data"`
	Dup      int    `json:"dup"`
}

type CompleteInfo struct {
	Mode CompleteMode `json:"mode"`
}
