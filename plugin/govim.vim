" Workaround for https://github.com/vim/vim/issues/4530
if exists("g:govimpluginloaded")
  finish
endif
let g:govimpluginloaded=1

augroup govim
augroup END

let s:minVimSafeState = has("patch-8.1.2056")

" TODO: we should source a code-generated, auto-loaded
" vim file or similar to source this minimum version
if !has("patch-8.1.1711")
  echoerr "Need at least version v8.1.1711 of Vim; govim will not be loaded"
  finish
endif

if has("patch-8.2.0452") && !has("patch-8.2.0466")
  echoerr "Vim versions v8.2.0452 <= N < v8.2.0466 have a bug that affects govim. Please update to another version"
  finish
endif

" TODO: we are ignoring windows right now....
let s:tmpdir = $TMPDIR
if s:tmpdir == ""
  let s:tmpdir = "/tmp"
endif
let s:filetmpl = $GOVIM_LOGFILE_TMPL
if s:filetmpl == ""
  let s:filetmpl = "%v_%v_%v"
endif
let s:filetmpl .= ".log"
let s:filetmpl = substitute(s:filetmpl, "%v", "vim_channel", "")
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
let s:govim_logfile="<unset>"
let s:gopls_logfile="<unset>"
call ch_logfile(s:ch_logfile, "a")
let s:channel = ""
let s:timer = ""
let s:plugindir = expand(expand("<sfile>:p:h:h"))

let s:govim_status = "loading"
let s:loadStatusCallbacks = []

let s:userBusy = 0

set ballooneval
set balloonevalterm

let s:waitingToDrain = 0
let s:scheduleBacklog = []
let s:activeGovimCalls = 0
augroup govimScheduler

function s:ch_evalexpr(args)
  " For all callbacks to govim (other than the handler ultimately responsible
  " for a listener_add callback) we need to flush any pending delta
  " notifications so that govim isn't ever working with stale buffer
  " contents
  if a:args[0] != "function" || a:args[1] != "function:GOVIM_internal_BufChanged"
    call listener_flush()
  endif
  if s:minVimSafeState
    let l:resp = ch_evalexpr(s:channel, a:args)
    if l:resp[0] != ""
      throw l:resp[0]
    endif
    return l:resp[1]
  endif

  let s:activeGovimCalls += 1
  let l:resp = ch_evalexpr(s:channel, a:args)
  let s:activeGovimCalls -= 1
  if l:resp[0] != ""
    throw l:resp[0]
  endif
  call s:drainScheduleBacklog(v:false)
  return l:resp[1]
endfunction

function s:schedule(id)
  call add(s:scheduleBacklog, a:id)
  " The only state('wxc') in which it is safe to run work immediately is 'c'.
  " Reason being, in state 's' the only active callback is the one processing
  " the received channel message (to schedule work). Anything else is unsafe,
  " so we must enqueue the work for later.
  "
  " More discussion at:
  "
  " https://groups.google.com/forum/#!topic/vim_dev/op_PKiE9iog
  "
  if s:minVimSafeState
    if state('cwx') != 'c'
      call ch_log("minVimSafeState: enqueuing work because state is ".string(state()))
      if !s:waitingToDrain
        au govimScheduler SafeState,SafeStateAgain * ++nested call s:drainScheduleBacklog(v:true)
        let s:waitingToDrain = 1
      endif
      return
    endif
    call ch_log("minVimSafeState: running work immediately because state is ".string(state()))
  endif
  call s:drainScheduleBacklog(v:false)
endfunction

function s:drainScheduleBacklog(drop)
  if s:minVimSafeState
    if a:drop
      au! govimScheduler SafeState,SafeStateAgain
    endif
  elseif s:activeGovimCalls != 0
    call ch_log("old safe state: cannot drain schedule backlog with pending calls")
    return
  else
    call ch_log("old safe state: will drain schedule backlog; no pending calls")
  endif
  while len(s:scheduleBacklog) > 0
    let l:args = ["schedule", s:scheduleBacklog[0]]
    let s:scheduleBacklog = s:scheduleBacklog[1:]
    let l:resp = s:ch_evalexpr(l:args)
  endwhile
  let s:waitingToDrain = 0
endfunction

function s:callbackFunction(name, args)
  let l:args = ["function", "function:".a:name, a:args]
  return s:ch_evalexpr(l:args)
endfunction

function s:callbackRangeFunction(name, first, last, args)
  let l:args = ["function", "function:".a:name, a:first, a:last, a:args]
  return s:ch_evalexpr(l:args)
endfunction

function s:callbackCommand(name, flags, ...)
  let l:args = ["function", "command:".a:name, a:flags]
  call extend(l:args, a:000)
  return s:ch_evalexpr(l:args)
endfunction

function s:callbackAutoCommand(name, def, exprs)
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
  let l:args = ["function", a:name, a:def, l:exprVals]
  return s:ch_evalexpr(l:args)
endfunction

function s:doShutdown()
  if s:govim_status != "loaded" && s:govim_status != "initcomplete"
    " TODO: anything to do here other than return?
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
    call GOVIM_internal_SetUserBusy(s:userBusy, s:cursorPos())
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
      if $GOVIM_DISABLE_USER_BUSY != "true"
        au govim CursorMoved,CursorMovedI * ++nested :call s:userBusy(1)
        au govim CursorHold,CursorHoldI * ++nested :call s:userBusy(0)
      endif
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
  execute "autocmd " . a:def . " call s:callbackAutoCommand(\"" . a:name . "\", \"".escape(a:def, '"')."\", [".join(l:exprStrings, ",")."])"
endfunction

func s:defineCommand(name, attrs)
  let l:def = "command! "
  let l:args = ""
  let l:flags = ['"mods": expand("<mods>")']
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
command -bar GOVIMLogfilePaths echom "Vim channel logfile: ".s:ch_logfile | echom "govim logfile: ".s:govim_logfile | echom "gopls logfile: ".s:gopls_logfile

function s:install(force)
  let oldpath = getcwd()
  execute "cd ".s:plugindir
  " TODO: make work on Windows
  let commit = trim(system("git rev-parse HEAD 2>&1"))
  if v:shell_error
    throw commit
  endif
  let targetdir = s:plugindir."/cmd/govim/.bin/".commit."/"
  if a:force || $GOVIM_ALWAYS_INSTALL == "true" || !filereadable(targetdir."govim") || !filereadable(targetdir."gopls")
    echom "Installing govim and gopls"
    call feedkeys(" ") " to prevent press ENTER to continue
    " TODO: make work on Windows
    let install = system("env GO111MODULE=on GOBIN=".shellescape(targetdir)." go install github.com/govim/govim/cmd/govim golang.org/x/tools/gopls 2>&1")
    if v:shell_error
      throw install
    endif
  endif
  execute "cd ".oldpath
  return targetdir
endfunction

" TODO: would be nice to be able to specify -1 as a timeout
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

" s:cursorPos returns a structure that represents the current cursor position.
" This is interpretted within (*vimstate).parseCursorPos
function s:cursorPos()
  let l:bufnr = bufnr("")
  let l:line = line(".")
  let l:col = col(".")
  let l:winnr = winnr()
  let l:winid = win_getid(l:winnr)
  let l:screenpos = screenpos(l:winid, l:line, l:col)
  return {"bufnr": l:bufnr,
        \ "line": l:line,
        \ "col": l:col,
        \ "winnr": l:winnr,
        \ "winid": l:winid,
        \ "screenpos": l:screenpos
        \ }
endfunction

function s:batchCall(calls)
  " calls is a [][]interface. Each call c in calls has the following structure:
  "
  " c[0] is the type; 'call' or 'expr'
  " c[1] is the must function to assert, e.g. s:mustBeZero
  "
  " For a call:
  " c[2] is the function name
  " c[3:-1] are the args to the function
  "
  " For an expr:
  " c[2] is the expression to evaluate
  "
  let l:results = []
  for l:call in a:calls
    let l:type = l:call[0]
    let l:mustName = l:call[1][0]
    let l:mustArgs = l:call[1][1]
    if type(l:mustArgs) == type(v:none)
      let l:mustArgs = []
    endif
    let Must = call(l:mustName, l:mustArgs)
    if l:type == "call"
      let l:fn = l:call[2]
      let l:args = l:call[3:-1]
      let F = function(l:fn, l:args)
      let l:res = v:none
      let l:err = v:none
      try
        let l:res = F()
      catch
        let l:err = v:exception
      endtry
      let l:check = Must(l:res, l:err)
      if !l:check[0]
        throw "failed to call ".l:fn."(".string(l:args)."): ".l:check[1]
      endif
    elseif l:type == "expr"
      let l:expr = l:call[2]
      let l:res = v:none
      let l:err = v:none
      try
        let l:res = eval(l:expr)
      catch
        let l:err = v:exception
      endtry
      let l:check = Must(l:res, l:err)
      if !l:check[0]
        throw "failed to eval ".l:expr.": ".l:check[1]
      endif
    else
      throw "Unknown batch type: ".l:type
    endif
    call add(l:results, l:res)
  endfor
  return l:results
endfunction

function s:mustNoError()
  let l:args = {}
  function l:args.f(v, err)
    if a:err != v:none
      return [v:false, a:err]
    endif
    return [v:true, ""]
  endfunction
  return l:args.f
endfunction

function s:mustBeZero()
  let l:args = {}
  function l:args.f(v, err)
    if a:err != v:none
      return [v:false, a:err]
    endif
    if a:v != 0
      return [v:false, "got non-zero return value"]
    endif
    return [v:true, ""]
  endfunction
  return l:args.f
endfunction

function s:mustBeErrorOrNil(...)
  let l:args = {'patterns': a:000}
  function l:args.f(v, err)
    if a:err == v:none
      return [v:true, ""]
    endif
    for l:v in self.patterns
      if match(a:err, l:v) >= 0
        call ch_log("Ignoring batch error: ".string(a:err))
        return [v:true, ""]
      endif
    endfor
    return [v:false, a:err]
  endfunction
  return l:args.f
endfunction

function GOVIM_internal_SuggestedFixesFilter(id, key)
    if a:key == "\<c-n>"
        GOVIMSuggestedFixes next
        return 1
    elseif a:key == "\<c-p>"
        GOVIMSuggestedFixes prev
        return 1
    endif

    return popup_filter_menu(a:id, a:key)
endfunc

" In case we are running in test mode
if $GOVIM_DISABLE_USER_BUSY == "true"
  function GOVIM_test_SetUserBusy(busy)
    return s:userBusy(a:busy)
  endfunction
endif
