// Code generated by "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source/genopts"; DO NOT EDIT.

package source

const OptionsJson = "{\"Debugging\":[{\"Name\":\"verboseOutput\",\"Type\":\"bool\",\"Doc\":\"verboseOutput enables additional debug logging.\\n\",\"EnumValues\":null,\"Default\":\"false\"},{\"Name\":\"completionBudget\",\"Type\":\"time.Duration\",\"Doc\":\"completionBudget is the soft latency goal for completion requests. Most\\nrequests finish in a couple milliseconds, but in some cases deep\\ncompletions can take much longer. As we use up our budget we\\ndynamically reduce the search scope to ensure we return timely\\nresults. Zero means unlimited.\\n\",\"EnumValues\":null,\"Default\":\"100000000\"},{\"Name\":\"literalCompletions\",\"Type\":\"bool\",\"Doc\":\"literalCompletions controls whether literal candidates such as\\n\\\"\\u0026someStruct{}\\\" are offered. Tests disable this flag to simplify\\ntheir expected values.\\n\",\"EnumValues\":null,\"Default\":\"true\"}],\"Experimental\":[{\"Name\":\"analyses\",\"Type\":\"map[string]bool\",\"Doc\":\"analyses specify analyses that the user would like to enable or disable.\\nA map of the names of analysis passes that should be enabled/disabled.\\nA full list of analyzers that gopls uses can be found [here](analyzers.md)\\n\\nExample Usage:\\n```json5\\n...\\n\\\"analyses\\\": {\\n  \\\"unreachable\\\": false, // Disable the unreachable analyzer.\\n  \\\"unusedparams\\\": true  // Enable the unusedparams analyzer.\\n}\\n...\\n```\\n\",\"EnumValues\":null,\"Default\":\"{}\"},{\"Name\":\"codelens\",\"Type\":\"map[string]bool\",\"Doc\":\"overrides the enabled/disabled state of various code lenses. Currently, we\\nsupport several code lenses:\\n\\n* `generate`: run `go generate` as specified by a `//go:generate` directive.\\n* `upgrade_dependency`: upgrade a dependency listed in a `go.mod` file.\\n* `test`: run `go test -run` for a test func.\\n* `gc_details`: Show the gc compiler's choices for inline analysis and escaping.\\n\\nExample Usage:\\n```json5\\n\\\"gopls\\\": {\\n...\\n  \\\"codelens\\\": {\\n    \\\"generate\\\": false,  // Don't run `go generate`.\\n    \\\"gc_details\\\": true  // Show a code lens toggling the display of gc's choices.\\n  }\\n...\\n}\\n```\\n\",\"EnumValues\":null,\"Default\":\"{\\\"gc_details\\\":false,\\\"generate\\\":true,\\\"regenerate_cgo\\\":true,\\\"tidy\\\":true,\\\"upgrade_dependency\\\":true,\\\"vendor\\\":true}\"},{\"Name\":\"completionDocumentation\",\"Type\":\"bool\",\"Doc\":\"completionDocumentation enables documentation with completion results.\\n\",\"EnumValues\":null,\"Default\":\"true\"},{\"Name\":\"completeUnimported\",\"Type\":\"bool\",\"Doc\":\"completeUnimported enables completion for packages that you do not currently import.\\n\",\"EnumValues\":null,\"Default\":\"true\"},{\"Name\":\"deepCompletion\",\"Type\":\"bool\",\"Doc\":\"deepCompletion If true, this turns on the ability to return completions from deep inside relevant entities, rather than just the locally accessible ones.\\n\\nConsider this example:\\n\\n```go\\npackage main\\n\\nimport \\\"fmt\\\"\\n\\ntype wrapString struct {\\n    str string\\n}\\n\\nfunc main() {\\n    x := wrapString{\\\"hello world\\\"}\\n    fmt.Printf(\\u003c\\u003e)\\n}\\n```\\n\\nAt the location of the `\\u003c\\u003e` in this program, deep completion would suggest the result `x.str`.\\n\",\"EnumValues\":null,\"Default\":\"true\"},{\"Name\":\"matcher\",\"Type\":\"enum\",\"Doc\":\"matcher sets the algorithm that is used when calculating completion candidates.\\n\",\"EnumValues\":[\"\\\"CaseInsensitive\\\"\",\"\\\"CaseSensitive\\\"\",\"\\\"Fuzzy\\\"\"],\"Default\":\"\\\"Fuzzy\\\"\"},{\"Name\":\"annotations\",\"Type\":\"map[string]bool\",\"Doc\":\"annotations suppress various kinds of optimization diagnostics\\nthat would be reported by the gc_details command.\\n * noNilcheck suppresses display of nilchecks.\\n * noEscape suppresses escape choices.\\n * noInline suppresses inlining choices.\\n * noBounds suppresses bounds checking diagnostics.\\n\",\"EnumValues\":null,\"Default\":\"{}\"},{\"Name\":\"staticcheck\",\"Type\":\"bool\",\"Doc\":\"staticcheck enables additional analyses from staticcheck.io.\\n\",\"EnumValues\":null,\"Default\":\"false\"},{\"Name\":\"symbolMatcher\",\"Type\":\"enum\",\"Doc\":\"symbolMatcher sets the algorithm that is used when finding workspace symbols.\\n\",\"EnumValues\":[\"\\\"CaseInsensitive\\\"\",\"\\\"CaseSensitive\\\"\",\"\\\"Fuzzy\\\"\"],\"Default\":\"\\\"Fuzzy\\\"\"},{\"Name\":\"symbolStyle\",\"Type\":\"enum\",\"Doc\":\"symbolStyle specifies what style of symbols to return in symbol requests.\\n\",\"EnumValues\":[\"\\\"Dynamic\\\"\",\"\\\"Full\\\"\",\"\\\"Package\\\"\"],\"Default\":\"\\\"Package\\\"\"},{\"Name\":\"linksInHover\",\"Type\":\"bool\",\"Doc\":\"linksInHover toggles the presence of links to documentation in hover.\\n\",\"EnumValues\":null,\"Default\":\"true\"},{\"Name\":\"tempModfile\",\"Type\":\"bool\",\"Doc\":\"tempModfile controls the use of the -modfile flag in Go 1.14.\\n\",\"EnumValues\":null,\"Default\":\"true\"},{\"Name\":\"importShortcut\",\"Type\":\"enum\",\"Doc\":\"importShortcut specifies whether import statements should link to\\ndocumentation or go to definitions.\\n\",\"EnumValues\":[\"\\\"Both\\\"\",\"\\\"Definition\\\"\",\"\\\"Link\\\"\"],\"Default\":\"\\\"Both\\\"\"},{\"Name\":\"verboseWorkDoneProgress\",\"Type\":\"bool\",\"Doc\":\"verboseWorkDoneProgress controls whether the LSP server should send\\nprogress reports for all work done outside the scope of an RPC.\\n\",\"EnumValues\":null,\"Default\":\"false\"},{\"Name\":\"expandWorkspaceToModule\",\"Type\":\"bool\",\"Doc\":\"expandWorkspaceToModule instructs `gopls` to expand the scope of the workspace to include the\\nmodules containing the workspace folders. Set this to false to avoid loading\\nyour entire module. This is particularly useful for those working in a monorepo.\\n\",\"EnumValues\":null,\"Default\":\"true\"},{\"Name\":\"experimentalWorkspaceModule\",\"Type\":\"bool\",\"Doc\":\"experimentalWorkspaceModule opts a user into the experimental support\\nfor multi-module workspaces.\\n\",\"EnumValues\":null,\"Default\":\"false\"}],\"User\":[{\"Name\":\"buildFlags\",\"Type\":\"[]string\",\"Doc\":\"buildFlags is the set of flags passed on to the build system when invoked.\\nIt is applied to queries like `go list`, which is used when discovering files.\\nThe most common use is to set `-tags`.\\n\",\"EnumValues\":null,\"Default\":\"[]\"},{\"Name\":\"env\",\"Type\":\"[]string\",\"Doc\":\"env adds environment variables to external commands run by `gopls`, most notably `go list`.\\n\",\"EnumValues\":null,\"Default\":\"[]\"},{\"Name\":\"hoverKind\",\"Type\":\"enum\",\"Doc\":\"hoverKind controls the information that appears in the hover text.\\nSingleLine and Structured are intended for use only by authors of editor plugins.\\n\",\"EnumValues\":[\"\\\"FullDocumentation\\\"\",\"\\\"NoDocumentation\\\"\",\"\\\"SingleLine\\\"\",\"\\\"Structured\\\"\",\"\\\"SynopsisDocumentation\\\"\"],\"Default\":\"\\\"FullDocumentation\\\"\"},{\"Name\":\"usePlaceholders\",\"Type\":\"bool\",\"Doc\":\"placeholders enables placeholders for function parameters or struct fields in completion responses.\\n\",\"EnumValues\":null,\"Default\":\"false\"},{\"Name\":\"linkTarget\",\"Type\":\"string\",\"Doc\":\"linkTarget controls where documentation links go.\\nIt might be one of:\\n\\n* `\\\"godoc.org\\\"`\\n* `\\\"pkg.go.dev\\\"`\\n\\nIf company chooses to use its own `godoc.org`, its address can be used as well.\\n\",\"EnumValues\":null,\"Default\":\"\\\"pkg.go.dev\\\"\"},{\"Name\":\"local\",\"Type\":\"string\",\"Doc\":\"local is the equivalent of the `goimports -local` flag, which puts imports beginning with this string after 3rd-party packages.\\nIt should be the prefix of the import path whose imports should be grouped separately.\\n\",\"EnumValues\":null,\"Default\":\"\\\"\\\"\"},{\"Name\":\"gofumpt\",\"Type\":\"bool\",\"Doc\":\"gofumpt indicates if we should run gofumpt formatting.\\n\",\"EnumValues\":null,\"Default\":\"false\"}]}"
