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

function! s:validQuickfixAutoDiagnosticsDisable(v)
  if type(a:v) != 0  && type(a:v) != 6
    return [v:false, "must be of type number or bool"
  endif
  return [v:true, ""]
endfunction

function! s:validQuickfixSignsDisable(v)
  if type(a:v) != 0  && type(a:v) != 6
    return [v:false, "must be of type number or bool"
  endif
  return [v:true, ""]
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

function! s:validCompletionDeepCompletionsDisable(v)
  if type(a:v) != 0  && type(a:v) != 6
    return [v:false, "must be of type number or bool"
  endif
  return [v:true, ""]
endfunction

function! s:validCompletionFuzzyMatchingDisable(v)
  if type(a:v) != 0  && type(a:v) != 6
    return [v:false, "must be of type number or bool"
  endif
  return [v:true, ""]
endfunction

let s:validators = {
      \ "FormatOnSave": function("s:validFormatOnSave"),
      \ "QuickfixAutoDiagnosticsDisable": function("s:validQuickfixAutoDiagnosticsDisable"),
      \ "CompletionDeepCompletionsDisable": function("s:validCompletionDeepCompletionsDisable"),
      \ "CompletionFuzzyMatchingDisable": function("s:validCompletionFuzzyMatchingDisable"),
      \ "QuickfixSignsDisable": function("s:validQuickfixSignsDisable"),
      \ "ExperimentalMouseTriggeredHoverPopupOptions": function("s:validExperimentalMouseTriggeredHoverPopupOptions"),
      \ "ExperimentalCursorTriggeredHoverPopupOptions": function("s:validExperimentalCursorTriggeredHoverPopupOptions"),
      \ }

call govim#config#Set("FormatOnSave", "goimports")
