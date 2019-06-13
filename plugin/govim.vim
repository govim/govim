" Workaround for https://github.com/vim/vim/issues/4530
if exists("g:govimpluginloaded")
  finish
endif
let g:govimpluginloaded=1

" TODO we should source a code-generated, auto-loaded
" vim file or similar to source this minimum version
if !has("patch-8.1.1158")
  echoerr "Need at least version v8.1.1158 of Vim; govim will not be loaded"
  finish
endif

" TODO we are ignoring windows right now....
let s:tmpdir = $TMPDIR
if s:tmpdir == ""
  let s:tmpdir = "/tmp"
endif
let s:filetmpl = $GOVIM_LOGFILE_TMPL
if s:filetmpl == ""
  let s:filetmpl = "%v_%v_%v"
endif
let s:filetmpl = substitute(s:filetmpl, "%v", "vim_channel_log", "")
let s:filetmpl = substitute(s:filetmpl, "%v", strftime("%Y%m%d_%H%M_%S"), "")
if s:filetmpl =~ "%v"
  let s:filetmpl = substitute(s:filetmpl, "%v", "XXXXXXXXXXXX", "")
  let s:filetmpl = system("mktemp ".s:tmpdir."/".s:filetmpl." 2>&1")
  if v:shell_error
    throw s:filetmpl
  endif
else
  let s:filetmpl = s:tmpdir."/".s:filetmpl
endif
let s:ch_logfile = trim(s:filetmpl)
call ch_logfile(s:ch_logfile, "a")
echom "Vim channel logfile: ".s:ch_logfile
call feedkeys(" ") " to prevent press ENTER to continue
let s:channel = ""
let s:timer = ""
let s:plugindir = expand(expand("<sfile>:p:h:h"))

let s:govim_status = "loading"
let s:loadStatusCallbacks = []

let s:userBusy = 0

set ballooneval
set balloonevalterm

function s:callbackFunction(name, args)
  let l:args = ["function", "function:".a:name, a:args]
  let l:resp = ch_evalexpr(s:channel, l:args)
  if l:resp[0] != ""
    throw l:resp[0]
  endif
  return l:resp[1]
endfunction

function s:callbackRangeFunction(name, first, last, args)
  let l:args = ["function", "function:".a:name, a:first, a:last, a:args]
  let l:resp = ch_evalexpr(s:channel, l:args)
  if l:resp[0] != ""
    throw l:resp[0]
  endif
  return l:resp[1]
endfunction

function s:callbackCommand(name, flags, ...)
  let l:args = ["function", "command:".a:name, a:flags]
  call extend(l:args, a:000)
  let l:resp = ch_evalexpr(s:channel, l:args)
  if l:resp[0] != ""
    throw l:resp[0]
  endif
  return l:resp[1]
endfunction

function s:callbackAutoCommand(name, exprs)
  " When govim is the process of loading, i.e. its Init(Govim) method is
  " called, we make a number of calls to Vim to register functions, commands
  " autocommands etc. In parallel to this, Vim is busily loading itself.
  " Therefore (and this has been observed), it's entirely possible that before
  " govim finishes its Init(Govim) that we receive callbacks for autocmd
  " events. We _have_ to ignore these, and rely on the fact that the doautoall
  " which is called once we are initcomplete will put everything in order.
  " It's conceivable that calls to functions/commands _are_ valid during this
  " phase, so we allow those (for now)
  if s:govim_status != "initcomplete"
    return
  endif
  let l:exprVals = []
  for e in a:exprs
    call add(l:exprVals, eval(e))
  endfor
  let l:args = ["function", a:name, l:exprVals]
  let l:resp = ch_evalexpr(s:channel, l:args)
  if l:resp[0] != ""
    throw l:resp[0]
  endif
  return l:resp[1]
endfunction

function s:doShutdown()
  if s:govim_status != "loaded" && s:govim_status != "initcomplete"
    " TODO anything to do here other than return?
    return
  endif
  call ch_close(s:channel)
endfunction

function s:buildCurrentViewport()
  let l:currTabNr = tabpagenr()
  let l:currWinNr = winnr()
  let l:currWin = {}
  let l:windows = []
  for l:w in getwininfo()
    let l:sw = filter(l:w, 'v:key != "variables"')
    call add(l:windows, l:sw)
    if l:sw.tabnr == l:currTabNr && l:sw.winnr == l:currWinNr
      let l:currWin = l:sw
    endif
  endfor
  let l:viewport = {
        \ 'Current': l:currWin,
        \ 'Windows': l:windows,
        \ }
  return l:viewport
endfunction

function GOVIMPluginStatus(...)
  if s:govim_status != "loaded" && s:govim_status != "failed" && len(a:000) != 0
    call extend(s:loadStatusCallbacks, a:000)
  endif
  return s:govim_status
endfunction

function s:userBusy(busy)
  if s:userBusy != a:busy
    let s:userBusy = a:busy
    call GOVIM_internal_SetUserBusy(s:userBusy)
  endif
endfunction

function s:define(channel, msg)
  " format is [type, ...]
  " type is function, command or autocmd
  try
    let l:id = a:msg[0]
    let l:resp = ["callback", l:id, [""]]
    if a:msg[1] == "loaded"
      let s:govim_status = "loaded"
      for F in s:loadStatusCallbacks
        call call(F, [s:govim_status])
      endfor
    elseif a:msg[1] == "initcomplete"
      let s:govim_status = "initcomplete"
      " doautoall BufRead also triggers ftplugin stuff
      doautoall BufRead
      doautoall FileType
      au CursorMoved,CursorMovedI *.go :call s:userBusy(1)
      au CursorHold,CursorHoldI *.go :call s:userBusy(0)
      for F in s:loadStatusCallbacks
        call call(F, [s:govim_status])
      endfor
    elseif a:msg[1] == "currentViewport"
      let l:res = s:buildCurrentViewport()
    elseif a:msg[1] == "function"
      call s:defineFunction(a:msg[2], a:msg[3], 0)
    elseif a:msg[1] == "rangefunction"
      call s:defineFunction(a:msg[2], a:msg[3], 1)
    elseif a:msg[1] == "command"
      call s:defineCommand(a:msg[2], a:msg[3])
    elseif a:msg[1] == "autocmd"
      call s:defineAutoCommand(a:msg[2], a:msg[3], a:msg[4])
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
    elseif a:msg[1] == "error"
      let l:msg = a:msg[2]
      " this is an async call from the client
      throw l:msg
      return
    else
      throw "unknown callback function type ".a:msg[1]
    endif
  catch
    let l:resp[2][0] = 'Caught ' . string(v:exception) . ' in ' . v:throwpoint
  endtry
  call ch_sendexpr(a:channel, l:resp)
endfunction

func s:defineAutoCommand(name, def, exprs)
  let l:exprStrings = []
  for e in a:exprs
    call add(l:exprStrings, '"'.escape(e, '"').'"')
  endfor
  execute "autocmd " . a:def . " call s:callbackAutoCommand(\"" . a:name . "\", [".join(l:exprStrings, ",")."])"
endfunction

func s:defineCommand(name, attrs)
  let l:def = "command! "
  let l:args = ""
  let l:flags = ['"mods": expand("<mods>")']
  " let l:flags = []
  if has_key(a:attrs, "nargs")
    let l:def .= " ". a:attrs["nargs"]
    if a:attrs["nargs"] != "-nargs=0"
      let l:args = ", <args>"
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
  if len(a:argsStr) == 1 && a:argsStr[0] == "..."
    let l:args = "let l:args = a:000\n"
  elseif len(a:argsStr) > 0
    let l:args = "let l:args = ["
    let l:join = ""
    for i in a:argsStr
      if i == "..."
        let l:args = l:args.l:join."a:000"
      else
        let l:args = l:args.l:join."a:".i
      endif
      let l:join = ", "
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

function s:govimExit(job, exitstatus)
  if a:exitstatus != 0
    let s:govim_status = "failed"
  else
    let s:govim_status = "exited"
  endif
  for i in s:loadStatusCallbacks
    call call(i, [s:govim_status])
  endfor
  if a:exitstatus != 0
    throw "govim plugin died :("
  endif
endfunction

command -bar GOVIMPluginInstall echom "Installed to ".s:install(1)

function s:install(force)
  let oldpath = getcwd()
  execute "cd ".s:plugindir
  " TODO make work on Windows
  let commit = trim(system("git rev-parse HEAD 2>&1"))
  if v:shell_error
    throw commit
  endif
  let targetdir = s:plugindir."/cmd/govim/.bin/".commit."/"
  if a:force || $GOVIM_ALWAYS_INSTALL == "true" || !filereadable(targetdir."govim") || !filereadable(targetdir."gopls")
    echom "Installing govim and gopls"
    call feedkeys(" ") " to prevent press ENTER to continue
    let oldgobin = $GOBIN
    let oldgomod = $GO111MODULE
    let $GO111MODULE = "on"
    let $GOBIN = targetdir
    " TODO make work on Windows
    let install = system("go install github.com/myitcv/govim/cmd/govim golang.org/x/tools/gopls 2>&1")
    if v:shell_error
      throw install
    endif
    let $GOBIN = oldgobin
    let $GO111MODULE=oldgomod
  endif
  execute "cd ".oldpath
  return targetdir
endfunction

" TODO - would be nice to be able to specify -1 as a timeout
let opts = {"in_mode": "json", "out_mode": "json", "err_mode": "json", "callback": function("s:define"), "timeout": 30000}
if $GOVIMTEST_SOCKET != ""
  let s:channel = ch_open($GOVIMTEST_SOCKET, opts)
else
  let targetdir = s:install(0)
  let start = $GOVIM_RUNCMD
  if start == ""
    let start = targetdir."govim ".targetdir."gopls"
  endif
  let opts.exit_cb = function("s:govimExit")
  let job = job_start(start, opts)
  let s:channel = job_getchannel(job)
endif

au VimLeave * call s:doShutdown()

function GOVIM_internal_EnrichDelta(bufnr, start, end, added, changes)
  for l:change in a:changes
    let l:change.lines = getbufline(a:bufnr, l:change.lnum, l:change.end-1+l:change.added)
  endfor
  call GOVIM_internal_BufChanged(a:bufnr, a:start, a:end, a:added, a:changes)
endfunction

function s:applyVimEdits(batch)
  for e in a:batch.Edits
    try | silent undojoin | catch | endtry
    if e.Type == "delete"
      call deletebufline(a:batch.BufNr, e.Start, e.End)
    elseif e.Type == "append"
      call appendbufline(a:batch.BufNr, e.Start, e.Lines)
    else
      throw "Unknown edit type ".e.Type
    endif
  endfor
  if a:batch.Flush
    call listener_flush(a:batch.BufNr)
  endif
endfunction
