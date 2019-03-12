" TODO work out why we need to set this
set nocompatible
set nobackup
set nowritebackup
set noswapfile

" Useful for logging
" call ch_logfile("/tmp/log.out", "a")

let s:channel = ""

function s:callbackFunction(args)
  let l:args = ["function"]
  call extend(l:args, a:args)
  return ch_evalexpr(s:channel, l:args)
endfunction

function s:define(channel, msg)
  " format is [type, ...]
  " type is function, command or autocmd
  try
    let l:id = a:msg[0]
    if a:msg[1] == "function"
      " define a function
      let l:name = a:msg[2]
      let l:fargs = a:msg[3]
      let l:params = join(l:fargs, ", ")
      if len(l:fargs) == 0
        let l:args = "let l:args = [\"".l:name."\"]\n"
      else
        let l:args = "let l:args = [\"".l:name.", "
        for i in l:fargs
          if i == "..."
            let l:args = l:args."a:000"
          else
            let l:args = l:args."a:".i
          endif
        endfor
        let l:args = l:args."]\n"
      endif
      execute "function! "  . l:name . "(" . l:params . ")\n" .
            \ l:args .
            \ "return s:callbackFunction(l:args)\n" .
            \ "endfunction\n"
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
    else
      throw "unknown definition type ".a:msg[0]
    endif
    " success
    call ch_sendexpr(a:channel, ["callback", l:id, ""])
  catch
    call ch_sendexpr(a:channel, ["callback", l:id, string(v:exception)])
  endtry
endfunction

let opts = {"in_mode": "json", "out_mode": "json", "err_mode": "json", "callback": function("s:define")}
if $GOVIMTEST_SOCKET != ""
  let s:channel = ch_open($GOVIMTEST_SOCKET, opts)
else
  let job = job_start(["gobin", "-m", "-run", "github.com/myitcv/govim/cmd/govim"], opts)
  let s:channel = job_getchannel(job)
endif
