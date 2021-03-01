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
	if v.HighlightReferences != nil {
		r.HighlightReferences = v.HighlightReferences
	}
	if v.HoverDiagnostics != nil {
		r.HoverDiagnostics = v.HoverDiagnostics
	}
	if v.CompletionDeepCompletions != nil {
		r.CompletionDeepCompletions = v.CompletionDeepCompletions
	}
	if v.CompletionMatcher != nil {
		r.CompletionMatcher = v.CompletionMatcher
	}
	if v.SymbolMatcher != nil {
		r.SymbolMatcher = v.SymbolMatcher
	}
	if v.SymbolStyle != nil {
		r.SymbolStyle = v.SymbolStyle
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
	if v.TempModfile != nil {
		r.TempModfile = v.TempModfile
	}
	if v.GoplsEnv != nil {
		r.GoplsEnv = v.GoplsEnv
	}
	if v.Analyses != nil {
		r.Analyses = v.Analyses
	}
	if v.OpenLastProgressWith != nil {
		r.OpenLastProgressWith = v.OpenLastProgressWith
	}
	if v.Gofumpt != nil {
		r.Gofumpt = v.Gofumpt
	}
	if v.ExperimentalAutoreadLoadedBuffers != nil {
		r.ExperimentalAutoreadLoadedBuffers = v.ExperimentalAutoreadLoadedBuffers
	}
	if v.ExperimentalMouseTriggeredHoverPopupOptions != nil {
		r.ExperimentalMouseTriggeredHoverPopupOptions = v.ExperimentalMouseTriggeredHoverPopupOptions
	}
	if v.ExperimentalCursorTriggeredHoverPopupOptions != nil {
		r.ExperimentalCursorTriggeredHoverPopupOptions = v.ExperimentalCursorTriggeredHoverPopupOptions
	}
	if v.ExperimentalWorkaroundCompleteoptLongest != nil {
		r.ExperimentalWorkaroundCompleteoptLongest = v.ExperimentalWorkaroundCompleteoptLongest
	}
	if v.ExperimentalProgressPopups != nil {
		r.ExperimentalProgressPopups = v.ExperimentalProgressPopups
	}
	if v.ExperimentalAllowModfileModifications != nil {
		r.ExperimentalAllowModfileModifications = v.ExperimentalAllowModfileModifications
	}
	if v.ExperimentalWorkspaceModule != nil {
		r.ExperimentalWorkspaceModule = v.ExperimentalWorkspaceModule
	}
}
