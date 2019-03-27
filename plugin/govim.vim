" Useful for debugging
" call ch_logfile("/tmp/vimchannel.out", "a")
let s:channel = ""
let s:timer = ""
let s:currViewport = {}

set mouse=a
set ttymouse=sgr
set balloondelay=250
set ballooneval
set balloonevalterm
syntax on

function s:callbackFunction(name, args)
  let l:args = ["function", "function:".a:name]
  call extend(l:args, a:args)
  let l:resp = ch_evalexpr(s:channel, l:args)
  if l:resp[0] != ""
    echoerr l:resp[0]
  endif
  return l:resp[1]
endfunction

function s:callbackRangeFunction(name, first, last, args)
  let l:args = ["function", "function:".a:name, a:first, a:last]
  call extend(l:args, a:args)
  let l:resp = ch_evalexpr(s:channel, l:args)
  if l:resp[0] != ""
    echoerr l:resp[0]
  endif
  return l:resp[1]
endfunction

function s:callbackCommand(name, flags, ...)
  let l:args = ["function", "command:".a:name, a:flags]
  call extend(l:args, a:000)
  let l:resp = ch_evalexpr(s:channel, l:args)
  if l:resp[0] != ""
    echoerr l:resp[0]
  endif
  return l:resp[1]
endfunction

function s:callbackAutoCommand(name)
  let l:args = ["function", a:name]
  let l:resp = ch_evalexpr(s:channel, l:args)
  if l:resp[0] != ""
    echoerr l:resp[0]
  endif
  return l:resp[1]
endfunction

function s:updateViewport(timer)
  let l:currTab = tabpagenr()
  let l:currWin = winnr()
  let l:windows = []
  for l:w in getwininfo()
    if l:w.tabnr != l:currTab
      continue
    endif
    let l:sw = filter(l:w, 'v:key != "variables"')
    call add(l:windows, l:sw)
  endfor
  let l:viewport = {
        \ 'currTab': l:currTab,
        \ 'currWin': l:currWin,
        \ 'windows': l:windows,
        \ }
  if s:currViewport != l:viewport
    let s:currViewport = l:viewport
    let l:resp = ch_evalexpr(s:channel, ["function", "govim:OnViewportChange", l:viewport])
    if l:resp[0] != ""
      " TODO disable the timer and the autocmd callback
      echoerr l:resp[0]
    endif
  endif
endfunction

function s:define(channel, msg)
  " format is [type, ...]
  " type is function, command or autocmd
  try
    let l:id = a:msg[0]
    let l:resp = ["callback", l:id, [""]]
    if a:msg[1] == "loaded"
      " the plugin has loaded, setup and well-known plugin-level
      " stuff like OnViewportChange
      let s:timer = timer_start(100, function('s:updateViewport'), {'repeat': -1})
      au CursorMoved,CursorMovedI,BufWinEnter * call s:updateViewport(0)
    elseif a:msg[1] == "function"
      call s:defineFunction(a:msg[2], a:msg[3], 0)
    elseif a:msg[1] == "rangefunction"
      call s:defineFunction(a:msg[2], a:msg[3], 1)
    elseif a:msg[1] == "command"
      call s:defineCommand(a:msg[2], a:msg[3])
    elseif a:msg[1] == "autocmd"
      call s:defineAutoCommand(a:msg[2], a:msg[3])
    elseif a:msg[1] == "redraw"
      let l:force = a:msg[2]
      let l:args = ""
      if l:force == "force"
        let l:args = '!'
      endif
      execute "redraw".l:args
    elseif a:msg[1] == "ex"
      let l:expr = a:msg[2]
      execute l:expr
    elseif a:msg[1] == "normal"
      let l:expr = a:msg[2]
      execute "normal ".l:expr
    elseif a:msg[1] == "expr"
      let l:expr = a:msg[2]
      let l:res = eval(l:expr)
      call add(l:resp[2], l:res)
    elseif a:msg[1] == "call"
      let l:fn = a:msg[2]
      let F= function(l:fn, a:msg[3:-1])
      let l:res = F()
      call add(l:resp[2], l:res)
    else
      throw "unknown callback function type ".a:msg[1]
    endif
  catch
    let l:resp[2][0] = 'Caught ' . string(v:exception) . ' in ' . v:throwpoint
  finally
    call ch_sendexpr(a:channel, l:resp)
  endtry
endfunction

func s:defineAutoCommand(name, def)
  execute "autocmd " . a:def . " call s:callbackAutoCommand(\"" . a:name . "\")"
endfunction

func s:defineCommand(name, attrs)
  let l:def = "command! "
  let l:args = ""
  let l:flags = ['"mods": split("<mods>", ",")']
  " let l:flags = []
  if has_key(a:attrs, "nargs")
    let l:def .= " ". a:attrs["nargs"]
    if a:attrs["nargs"] != "-nargs=0"
      let l:args = ", <f-args>"
    endif
  endif
  if has_key(a:attrs, "range")
    let l:def .= " ".a:attrs["range"]
    call add(l:flags, '"line1": <line1>')
    call add(l:flags, '"line2": <line2>')
    call add(l:flags, '"range": <range>')
  endif
  if has_key(a:attrs, "count")
    let l:def .= " ". a:attrs["count"]
    call add(l:flags, '"count": <count>')
  endif
  if has_key(a:attrs, "complete")
    let l:def .= " ". a:attrs["complete"]
  endif
  if has_key(a:attrs, "general")
    for l:a in a:attrs["general"]
      let l:def .= " ". l:a
      if l:a == "-bang"
        call add(l:flags, '"bang": "<bang>"')
      endif
      if l:a == "-register"
        call add(l:flags, '"register": "<reg>"')
      endif
    endfor
  endif
  let l:flagsStr = "{" . join(l:flags, ", ") . "}"
  let l:def .= " " . a:name . " call s:callbackCommand(\"". a:name . "\", " . l:flagsStr . l:args . ")"
  execute l:def
endfunction

func s:defineFunction(name, argsStr, range)
  let l:params = join(a:argsStr, ", ")
  let l:args = "let l:args = []\n"
  if len(a:argsStr) > 0
    let l:args = "let l:args = ["
    for i in a:argsStr
      if i == "..."
        let l:args = l:args."a:000"
      else
        let l:args = l:args."a:".i
      endif
    endfor
    let l:args = l:args."]"
  endif
  if a:range == 1
    let l:range = " range"
    let l:ret = "return s:callbackRangeFunction(\"" . a:name . "\", a:firstline, a:lastline, l:args)"
  else
    let l:range = ""
    let l:ret = "return s:callbackFunction(\"" . a:name . "\", l:args)"
  endif
  execute "function! "  . a:name . "(" . l:params . ") " . l:range . "\n" .
        \ l:args . "\n" .
        \ l:ret . "\n" .
        \ "endfunction\n"
endfunction

" TODO - would be nice to be able to specify -1 as a timeout
let opts = {"in_mode": "json", "out_mode": "json", "err_mode": "json", "callback": function("s:define"), "timeout": 30000}
if $GOVIMTEST_SOCKET != ""
  let s:channel = ch_open($GOVIMTEST_SOCKET, opts)
else
  let start = $GOVIM_RUNCMD
  if start == ""
    let start = ["gobin", "-m", "-run", "github.com/myitcv/govim/cmd/govim"]
  endif
  let opts.cwd = expand(expand("<sfile>:h"))
  let job = job_start(start, opts)
  let s:channel = job_getchannel(job)
endif
