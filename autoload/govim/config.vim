let s:config = {}
let s:func=0

function! govim#config#Set(key,value)
  if a:key == "_internal_Func"
    let s:func=a:value
  else
    if !has_key(s:validators, a:key)
      throw "Tried to set config for invalid key ".a:key
    endif
    let Func = s:validators[a:key]
    let validRes = call(Func, [a:value])
    if !validRes[0]
      throw "Tried to set invalid value for key ".a:key.": ".validRes[1]
    endif
    let s:config[a:key] = a:value
  endif
  call s:pushConfig()
endfunction

function! govim#config#Unset(key)
  let s:config = remove(s:config, a:key)
  call s:pushConfig()
endfunction

function! govim#config#Get()
  return copy(s:config)
endfunction

function! s:pushConfig()
  if s:func != 0
    let Func = s:func
    call call(Func, [s:config])
  endif
endfunction

function! s:validFormatOnSave(v)
  let valid = ["", "gofmt", "goimports"]
  if index(valid, a:v) < 0
    return [v:false, "must be one of: ".string(valid)]
  endif
  return [v:true, ""]
endfunction

function! s:validBool(v)
  if type(a:v) != 0  && type(a:v) != 6
    return [v:false, "must be of type number or bool"
  endif
  return [v:true, ""]
endfunction

function! s:validString(v)
  if type(a:v) != 1
    return [v:false, "must be of type string"
  endif
  return [v:true, ""]
endfunction

function! s:validQuickfixAutoDiagnostics(v)
  return s:validBool(a:v)
endfunction

function! s:validQuickfixSigns(v)
  return s:validBool(a:v)
endfunction

function! s:validHighlightDiagnostics(v)
    return s:validBool(a:v)
endfunction

function! s:validHighlightReferences(v)
    return s:validBool(a:v)
endfunction

function! s:validHoverDiagnostics(v)
    return s:validBool(a:v)
endfunction

function! s:validCompletionDeepCompletions(v)
  return s:validBool(a:v)
endfunction

function! s:validCompletionMatcher(v)
  let valid = ["caseInsensitive", "caseSensitive", "fuzzy"]
  if index(valid, a:v) < 0
    return [v:false, "must be one of: ".string(valid)]
  endif
  return [v:true, ""]
endfunction

function! s:validStaticcheck(v)
  return s:validBool(a:v)
endfunction

function! s:validCompleteUnimported(v)
  return s:validBool(a:v)
endfunction

function! s:validGoImportsLocalPrefix(v)
  return s:validString(a:v)
endfunction

function! s:validCompletionBudget(v)
  return s:validString(a:v)
endfunction

function! s:validTempModfile(v)
  return s:validBool(a:v)
endfunction

function! s:validGoplsEnv(v)
  if type(a:v) != 4
    return [v:false, "value must be a dict"]
  endif
  for [key, value] in items(a:v)
    if type(value) != 1
      return [v:false, "value for key ".key." must be a string"]
    endif
  endfor
  return [v:true, ""]
endfunction

function! s:validExperimentalAutoreadLoadedBuffers(v)
  return s:validBool(a:v)
endfunction

function! s:validExperimentalMouseTriggeredHoverPopupOptions(v)
  if has_key(a:v, "line")
    if type(a:v["line"]) != 0
      return [v:false, "line option must be of type number"]
    endif
  endif
  if has_key(a:v, "col")
    if type(a:v["col"]) != 0
      return [v:false, "col option must be of type number"]
    endif
  endif
  return [v:true, ""]
endfunction

function! s:validExperimentalCursorTriggeredHoverPopupOptions(v)
  return s:validExperimentalMouseTriggeredHoverPopupOptions(a:v)
endfunction

let s:validators = {
      \ "FormatOnSave": function("s:validFormatOnSave"),
      \ "QuickfixAutoDiagnostics": function("s:validQuickfixAutoDiagnostics"),
      \ "CompletionDeepCompletions": function("s:validCompletionDeepCompletions"),
      \ "CompletionMatcher": function("s:validCompletionMatcher"),
      \ "QuickfixSigns": function("s:validQuickfixSigns"),
      \ "HighlightDiagnostics": function("s:validHighlightDiagnostics"),
      \ "HighlightReferences": function("s:validHighlightReferences"),
      \ "HoverDiagnostics": function("s:validHoverDiagnostics"),
      \ "Staticcheck": function("s:validStaticcheck"),
      \ "CompleteUnimported": function("s:validCompleteUnimported"),
      \ "GoImportsLocalPrefix": function("s:validGoImportsLocalPrefix"),
      \ "CompletionBudget": function("s:validCompletionBudget"),
      \ "TempModfile": function("s:validTempModfile"),
      \ "GoplsEnv": function("s:validGoplsEnv"),
      \ "ExperimentalAutoreadLoadedBuffers": function("s:validExperimentalAutoreadLoadedBuffers"),
      \ "ExperimentalMouseTriggeredHoverPopupOptions": function("s:validExperimentalMouseTriggeredHoverPopupOptions"),
      \ "ExperimentalCursorTriggeredHoverPopupOptions": function("s:validExperimentalCursorTriggeredHoverPopupOptions"),
      \ }
