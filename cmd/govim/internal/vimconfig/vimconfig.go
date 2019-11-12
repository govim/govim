// Package vimconfig defines the mapping between Vim-specified config and
// govim config
package vimconfig

import (
	"github.com/govim/govim/cmd/govim/config"
)

type VimConfig struct {
	FormatOnSave                                 *config.FormatOnSave
	QuickfixAutoDiagnostics                      *int
	QuickfixSigns                                *int
	CompletionDeepCompletions                    *int
	CompletionFuzzyMatching                      *int
	Staticcheck                                  *int
	CompletionCaseSensitive                      *int
	CompleteUnimported                           *int
	ExperimentalMouseTriggeredHoverPopupOptions  *map[string]interface{}
	ExperimentalCursorTriggeredHoverPopupOptions *map[string]interface{}
}

func (c *VimConfig) ToConfig(d config.Config) config.Config {
	v := config.Config{
		FormatOnSave:                                 c.FormatOnSave,
		QuickfixSigns:                                boolVal(c.QuickfixSigns, d.QuickfixSigns),
		QuickfixAutoDiagnostics:                      boolVal(c.QuickfixAutoDiagnostics, d.QuickfixAutoDiagnostics),
		CompletionDeepCompletions:                    boolVal(c.CompletionDeepCompletions, d.CompletionDeepCompletions),
		CompletionFuzzyMatching:                      boolVal(c.CompletionFuzzyMatching, d.CompletionFuzzyMatching),
		Staticcheck:                                  boolVal(c.Staticcheck, d.Staticcheck),
		CompletionCaseSensitive:                      boolVal(c.CompletionCaseSensitive, d.CompletionCaseSensitive),
		CompleteUnimported:                           boolVal(c.CompleteUnimported, d.CompleteUnimported),
		ExperimentalMouseTriggeredHoverPopupOptions:  copyMap(c.ExperimentalMouseTriggeredHoverPopupOptions, d.ExperimentalMouseTriggeredHoverPopupOptions),
		ExperimentalCursorTriggeredHoverPopupOptions: copyMap(c.ExperimentalCursorTriggeredHoverPopupOptions, d.ExperimentalCursorTriggeredHoverPopupOptions),
	}
	if v.FormatOnSave == nil {
		v.FormatOnSave = d.FormatOnSave
	}
	return v
}

func boolVal(i *int, j *bool) *bool {
	if i == nil {
		return j
	}
	b := *i != 0
	return &b
}

func copyMap(i, j *map[string]interface{}) *map[string]interface{} {
	toCopy := i
	if i == nil {
		toCopy = j
		if j == nil {
			return nil
		}
	}
	res := make(map[string]interface{})
	for ck, cv := range *toCopy {
		res[ck] = cv
	}
	return &res
}

func FormatOnSaveVal(v config.FormatOnSave) *config.FormatOnSave {
	return &v
}

func BoolVal(v bool) *bool {
	return &v
}

func MapVal(v map[string]interface{}) *map[string]interface{} {
	return &v
}

// EqualBool returns true iff i and j are both nil, or if both are non-nil and
// dereference to the same bool value. Otherwise it returns false.
func EqualBool(i, j *bool) bool {
	if i == nil && j == nil {
		return true
	}
	if i == nil && j != nil ||
		i != nil && j == nil {
		return false
	}
	return *i == *j
}
