// Code generated by "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/source/genapijson"; DO NOT EDIT.

package source

const GeneratedAPIJSON = "{\"Options\":{\"Debugging\":[{\"Name\":\"verboseOutput\",\"Type\":\"bool\",\"Doc\":\"verboseOutput enables additional debug logging.\\n\",\"EnumValues\":null,\"Default\":\"false\"},{\"Name\":\"completionBudget\",\"Type\":\"time.Duration\",\"Doc\":\"completionBudget is the soft latency goal for completion requests. Most\\nrequests finish in a couple milliseconds, but in some cases deep\\ncompletions can take much longer. As we use up our budget we\\ndynamically reduce the search scope to ensure we return timely\\nresults. Zero means unlimited.\\n\",\"EnumValues\":null,\"Default\":\"\\\"100ms\\\"\"}],\"Experimental\":[{\"Name\":\"analyses\",\"Type\":\"map[string]bool\",\"Doc\":\"analyses specify analyses that the user would like to enable or disable.\\nA map of the names of analysis passes that should be enabled/disabled.\\nA full list of analyzers that gopls uses can be found [here](analyzers.md)\\n\\nExample Usage:\\n```json5\\n...\\n\\\"analyses\\\": {\\n  \\\"unreachable\\\": false, // Disable the unreachable analyzer.\\n  \\\"unusedparams\\\": true  // Enable the unusedparams analyzer.\\n}\\n...\\n```\\n\",\"EnumValues\":null,\"Default\":\"{}\"},{\"Name\":\"codelens\",\"Type\":\"map[string]bool\",\"Doc\":\"codelens overrides the enabled/disabled state of code lenses. See the \\\"Code Lenses\\\"\\nsection of settings.md for the list of supported lenses.\\n\\nExample Usage:\\n```json5\\n\\\"gopls\\\": {\\n...\\n  \\\"codelens\\\": {\\n    \\\"generate\\\": false,  // Don't show the `go generate` lens.\\n    \\\"gc_details\\\": true  // Show a code lens toggling the display of gc's choices.\\n  }\\n...\\n}\\n```\\n\",\"EnumValues\":null,\"Default\":\"{\\\"gc_details\\\":false,\\\"generate\\\":true,\\\"regenerate_cgo\\\":true,\\\"tidy\\\":true,\\\"upgrade_dependency\\\":true,\\\"vendor\\\":true}\"},{\"Name\":\"completionDocumentation\",\"Type\":\"bool\",\"Doc\":\"completionDocumentation enables documentation with completion results.\\n\",\"EnumValues\":null,\"Default\":\"true\"},{\"Name\":\"completeUnimported\",\"Type\":\"bool\",\"Doc\":\"completeUnimported enables completion for packages that you do not currently import.\\n\",\"EnumValues\":null,\"Default\":\"true\"},{\"Name\":\"deepCompletion\",\"Type\":\"bool\",\"Doc\":\"deepCompletion enables the ability to return completions from deep inside relevant entities, rather than just the locally accessible ones.\\n\\nConsider this example:\\n\\n```go\\npackage main\\n\\nimport \\\"fmt\\\"\\n\\ntype wrapString struct {\\n    str string\\n}\\n\\nfunc main() {\\n    x := wrapString{\\\"hello world\\\"}\\n    fmt.Printf(\\u003c\\u003e)\\n}\\n```\\n\\nAt the location of the `\\u003c\\u003e` in this program, deep completion would suggest the result `x.str`.\\n\",\"EnumValues\":null,\"Default\":\"true\"},{\"Name\":\"matcher\",\"Type\":\"enum\",\"Doc\":\"matcher sets the algorithm that is used when calculating completion candidates.\\n\",\"EnumValues\":[{\"Value\":\"\\\"CaseInsensitive\\\"\",\"Doc\":\"\"},{\"Value\":\"\\\"CaseSensitive\\\"\",\"Doc\":\"\"},{\"Value\":\"\\\"Fuzzy\\\"\",\"Doc\":\"\"}],\"Default\":\"\\\"Fuzzy\\\"\"},{\"Name\":\"annotations\",\"Type\":\"map[string]bool\",\"Doc\":\"annotations suppress various kinds of optimization diagnostics\\nthat would be reported by the gc_details command.\\n * noNilcheck suppresses display of nilchecks.\\n * noEscape suppresses escape choices.\\n * noInline suppresses inlining choices.\\n * noBounds suppresses bounds checking diagnostics.\\n\",\"EnumValues\":null,\"Default\":\"{}\"},{\"Name\":\"staticcheck\",\"Type\":\"bool\",\"Doc\":\"staticcheck enables additional analyses from staticcheck.io.\\n\",\"EnumValues\":null,\"Default\":\"false\"},{\"Name\":\"symbolMatcher\",\"Type\":\"enum\",\"Doc\":\"symbolMatcher sets the algorithm that is used when finding workspace symbols.\\n\",\"EnumValues\":[{\"Value\":\"\\\"CaseInsensitive\\\"\",\"Doc\":\"\"},{\"Value\":\"\\\"CaseSensitive\\\"\",\"Doc\":\"\"},{\"Value\":\"\\\"Fuzzy\\\"\",\"Doc\":\"\"}],\"Default\":\"\\\"Fuzzy\\\"\"},{\"Name\":\"symbolStyle\",\"Type\":\"enum\",\"Doc\":\"symbolStyle controls how symbols are qualified in symbol responses.\\n\\nExample Usage:\\n```json5\\n\\\"gopls\\\": {\\n...\\n  \\\"symbolStyle\\\": \\\"dynamic\\\",\\n...\\n}\\n```\\n\",\"EnumValues\":[{\"Value\":\"\\\"Dynamic\\\"\",\"Doc\":\"`\\\"Dynamic\\\"` uses whichever qualifier results in the highest scoring\\nmatch for the given symbol query. Here a \\\"qualifier\\\" is any \\\"/\\\" or \\\".\\\"\\ndelimited suffix of the fully qualified symbol. i.e. \\\"to/pkg.Foo.Field\\\" or\\njust \\\"Foo.Field\\\".\\n\"},{\"Value\":\"\\\"Full\\\"\",\"Doc\":\"`\\\"Full\\\"` is fully qualified symbols, i.e.\\n\\\"path/to/pkg.Foo.Field\\\".\\n\"},{\"Value\":\"\\\"Package\\\"\",\"Doc\":\"`\\\"Package\\\"` is package qualified symbols i.e.\\n\\\"pkg.Foo.Field\\\".\\n\"}],\"Default\":\"\\\"Package\\\"\"},{\"Name\":\"linksInHover\",\"Type\":\"bool\",\"Doc\":\"linksInHover toggles the presence of links to documentation in hover.\\n\",\"EnumValues\":null,\"Default\":\"true\"},{\"Name\":\"tempModfile\",\"Type\":\"bool\",\"Doc\":\"tempModfile controls the use of the -modfile flag in Go 1.14.\\n\",\"EnumValues\":null,\"Default\":\"true\"},{\"Name\":\"importShortcut\",\"Type\":\"enum\",\"Doc\":\"importShortcut specifies whether import statements should link to\\ndocumentation or go to definitions.\\n\",\"EnumValues\":[{\"Value\":\"\\\"Both\\\"\",\"Doc\":\"\"},{\"Value\":\"\\\"Definition\\\"\",\"Doc\":\"\"},{\"Value\":\"\\\"Link\\\"\",\"Doc\":\"\"}],\"Default\":\"\\\"Both\\\"\"},{\"Name\":\"verboseWorkDoneProgress\",\"Type\":\"bool\",\"Doc\":\"verboseWorkDoneProgress controls whether the LSP server should send\\nprogress reports for all work done outside the scope of an RPC.\\n\",\"EnumValues\":null,\"Default\":\"false\"},{\"Name\":\"semanticTokens\",\"Type\":\"bool\",\"Doc\":\"semanticTokens controls whether the LSP server will send\\nsemantic tokens to the client.\\n\",\"EnumValues\":null,\"Default\":\"false\"},{\"Name\":\"expandWorkspaceToModule\",\"Type\":\"bool\",\"Doc\":\"expandWorkspaceToModule instructs `gopls` to expand the scope of the workspace to include the\\nmodules containing the workspace folders. Set this to false to avoid loading\\nyour entire module. This is particularly useful for those working in a monorepo.\\n\",\"EnumValues\":null,\"Default\":\"true\"},{\"Name\":\"experimentalWorkspaceModule\",\"Type\":\"bool\",\"Doc\":\"experimentalWorkspaceModule opts a user into the experimental support\\nfor multi-module workspaces.\\n\",\"EnumValues\":null,\"Default\":\"false\"},{\"Name\":\"experimentalDiagnosticsDelay\",\"Type\":\"time.Duration\",\"Doc\":\"experimentalDiagnosticsDelay controls the amount of time that gopls waits\\nafter the most recent file modification before computing deep diagnostics.\\nSimple diagnostics (parsing and type-checking) are always run immediately\\non recently modified packages.\\n\\nThis option must be set to a valid duration string, for example `\\\"250ms\\\"`.\\n\",\"EnumValues\":null,\"Default\":\"\\\"0s\\\"\"},{\"Name\":\"experimentalPackageCacheKey\",\"Type\":\"bool\",\"Doc\":\"experimentalPackageCacheKey controls whether to use a coarser cache key\\nfor package type information to increase cache hits. This setting removes\\nthe user's environment, build flags, and working directory from the cache\\nkey, which should be a safe change as all relevant inputs into the type\\nchecking pass are already hashed into the key. This is temporarily guarded\\nby an experiment because caching behavior is subtle and difficult to\\ncomprehensively test.\\n\",\"EnumValues\":null,\"Default\":\"false\"}],\"User\":[{\"Name\":\"buildFlags\",\"Type\":\"[]string\",\"Doc\":\"buildFlags is the set of flags passed on to the build system when invoked.\\nIt is applied to queries like `go list`, which is used when discovering files.\\nThe most common use is to set `-tags`.\\n\",\"EnumValues\":null,\"Default\":\"[]\"},{\"Name\":\"env\",\"Type\":\"map[string]string\",\"Doc\":\"env adds environment variables to external commands run by `gopls`, most notably `go list`.\\n\",\"EnumValues\":null,\"Default\":\"{}\"},{\"Name\":\"hoverKind\",\"Type\":\"enum\",\"Doc\":\"hoverKind controls the information that appears in the hover text.\\nSingleLine and Structured are intended for use only by authors of editor plugins.\\n\",\"EnumValues\":[{\"Value\":\"\\\"FullDocumentation\\\"\",\"Doc\":\"\"},{\"Value\":\"\\\"NoDocumentation\\\"\",\"Doc\":\"\"},{\"Value\":\"\\\"SingleLine\\\"\",\"Doc\":\"\"},{\"Value\":\"\\\"Structured\\\"\",\"Doc\":\"`\\\"Structured\\\"` is an experimental setting that returns a structured hover format.\\nThis format separates the signature from the documentation, so that the client\\ncan do more manipulation of these fields.\\n\\nThis should only be used by clients that support this behavior.\\n\"},{\"Value\":\"\\\"SynopsisDocumentation\\\"\",\"Doc\":\"\"}],\"Default\":\"\\\"FullDocumentation\\\"\"},{\"Name\":\"usePlaceholders\",\"Type\":\"bool\",\"Doc\":\"placeholders enables placeholders for function parameters or struct fields in completion responses.\\n\",\"EnumValues\":null,\"Default\":\"false\"},{\"Name\":\"linkTarget\",\"Type\":\"string\",\"Doc\":\"linkTarget controls where documentation links go.\\nIt might be one of:\\n\\n* `\\\"godoc.org\\\"`\\n* `\\\"pkg.go.dev\\\"`\\n\\nIf company chooses to use its own `godoc.org`, its address can be used as well.\\n\",\"EnumValues\":null,\"Default\":\"\\\"pkg.go.dev\\\"\"},{\"Name\":\"local\",\"Type\":\"string\",\"Doc\":\"local is the equivalent of the `goimports -local` flag, which puts imports beginning with this string after 3rd-party packages.\\nIt should be the prefix of the import path whose imports should be grouped separately.\\n\",\"EnumValues\":null,\"Default\":\"\\\"\\\"\"},{\"Name\":\"gofumpt\",\"Type\":\"bool\",\"Doc\":\"gofumpt indicates if we should run gofumpt formatting.\\n\",\"EnumValues\":null,\"Default\":\"false\"}]},\"Commands\":[{\"Command\":\"gopls.generate\",\"Title\":\"Run go generate\",\"Doc\":\"generate runs `go generate` for a given directory.\\n\"},{\"Command\":\"gopls.fill_struct\",\"Title\":\"Fill struct\",\"Doc\":\"fill_struct is a gopls command to fill a struct with default\\nvalues.\\n\"},{\"Command\":\"gopls.regenerate_cgo\",\"Title\":\"Regenerate cgo\",\"Doc\":\"regenerate_cgo regenerates cgo definitions.\\n\"},{\"Command\":\"gopls.test\",\"Title\":\"Run test(s)\",\"Doc\":\"test runs `go test` for a specific test function.\\n\"},{\"Command\":\"gopls.tidy\",\"Title\":\"Run go mod tidy\",\"Doc\":\"tidy runs `go mod tidy` for a module.\\n\"},{\"Command\":\"gopls.undeclared_name\",\"Title\":\"Undeclared name\",\"Doc\":\"undeclared_name adds a variable declaration for an undeclared\\nname.\\n\"},{\"Command\":\"gopls.upgrade_dependency\",\"Title\":\"Upgrade dependency\",\"Doc\":\"upgrade_dependency upgrades a dependency.\\n\"},{\"Command\":\"gopls.vendor\",\"Title\":\"Run go mod vendor\",\"Doc\":\"vendor runs `go mod vendor` for a module.\\n\"},{\"Command\":\"gopls.extract_variable\",\"Title\":\"Extract to variable\",\"Doc\":\"extract_variable extracts an expression to a variable.\\n\"},{\"Command\":\"gopls.extract_function\",\"Title\":\"Extract to function\",\"Doc\":\"extract_function extracts statements to a function.\\n\"},{\"Command\":\"gopls.gc_details\",\"Title\":\"Toggle gc_details\",\"Doc\":\"gc_details controls calculation of gc annotations.\\n\"},{\"Command\":\"gopls.generate_gopls_mod\",\"Title\":\"Generate gopls.mod\",\"Doc\":\"generate_gopls_mod (re)generates the gopls.mod file.\\n\"}],\"Lenses\":[{\"Lens\":\"generate\",\"Title\":\"Run go generate\",\"Doc\":\"generate runs `go generate` for a given directory.\\n\"},{\"Lens\":\"regenerate_cgo\",\"Title\":\"Regenerate cgo\",\"Doc\":\"regenerate_cgo regenerates cgo definitions.\\n\"},{\"Lens\":\"test\",\"Title\":\"Run test(s)\",\"Doc\":\"test runs `go test` for a specific test function.\\n\"},{\"Lens\":\"tidy\",\"Title\":\"Run go mod tidy\",\"Doc\":\"tidy runs `go mod tidy` for a module.\\n\"},{\"Lens\":\"upgrade_dependency\",\"Title\":\"Upgrade dependency\",\"Doc\":\"upgrade_dependency upgrades a dependency.\\n\"},{\"Lens\":\"vendor\",\"Title\":\"Run go mod vendor\",\"Doc\":\"vendor runs `go mod vendor` for a module.\\n\"},{\"Lens\":\"gc_details\",\"Title\":\"Toggle gc_details\",\"Doc\":\"gc_details controls calculation of gc annotations.\\n\"}]}"
