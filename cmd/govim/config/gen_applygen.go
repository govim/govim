package config

func (r *Config) Apply(v *Config) {
	if v.FormatOnSave != nil {
		r.FormatOnSave = v.FormatOnSave
	}
	if v.QuickfixAutoDiagnostics != nil {
		r.QuickfixAutoDiagnostics = v.QuickfixAutoDiagnostics
	}
	if v.QuickfixSigns != nil {
		r.QuickfixSigns = v.QuickfixSigns
	}
	if v.HighlightDiagnostics != nil {
		r.HighlightDiagnostics = v.HighlightDiagnostics
	}
	if v.CompletionDeepCompletions != nil {
		r.CompletionDeepCompletions = v.CompletionDeepCompletions
	}
	if v.CompletionMatcher != nil {
		r.CompletionMatcher = v.CompletionMatcher
	}
	if v.Staticcheck != nil {
		r.Staticcheck = v.Staticcheck
	}
	if v.CompleteUnimported != nil {
		r.CompleteUnimported = v.CompleteUnimported
	}
	if v.GoImportsLocalPrefix != nil {
		r.GoImportsLocalPrefix = v.GoImportsLocalPrefix
	}
	if v.CompletionBudget != nil {
		r.CompletionBudget = v.CompletionBudget
	}
	if v.ExperimentalMouseTriggeredHoverPopupOptions != nil {
		r.ExperimentalMouseTriggeredHoverPopupOptions = v.ExperimentalMouseTriggeredHoverPopupOptions
	}
	if v.ExperimentalCursorTriggeredHoverPopupOptions != nil {
		r.ExperimentalCursorTriggeredHoverPopupOptions = v.ExperimentalCursorTriggeredHoverPopupOptions
	}
}
