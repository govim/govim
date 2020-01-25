" GOVIMTest_getqflist is a simpler wrapper around getqflist that substitutes
" bufname for bufnr
function! GOVIMTest_getqflist(...)
  return map(call(function('getqflist'), a:000), function('s:addbufname'))
endfunction

" GOVIMTest_sign_getplaced is a simple wrapper around sign_getplaced that
" substitutes bufname for bufnr
function! GOVIMTest_sign_getplaced(...)
  return map(call(function('sign_getplaced'), a:000), function('s:addbufname'))
endfunction

function! s:addbufname(key, val)
  let a:val.bufname = bufname(a:val.bufnr)
  unlet a:val.bufnr
  return a:val
endfunction

