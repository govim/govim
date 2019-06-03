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
    if !call(Func, [a:value])
      throw "Tried to set invalid value for key ".a:key.": ".a:value
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
  return index(["", "gofmt", "goimports"], a:v) >= 0
endfunction

function! s:validQuickfixAutoDiagnosticsDisable(v)
  return type(a:v) == 0
endfunction

let s:validators = {
      \ "FormatOnSave": function("s:validFormatOnSave"),
      \ "QuickfixAutoDiagnosticsDisable": function("s:validQuickfixAutoDiagnosticsDisable"),
      \ }

call govim#config#Set("FormatOnSave", "goimports")
