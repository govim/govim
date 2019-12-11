if exists("b:did_indent")
  finish
endif
let b:did_indent = 1

setlocal indentexpr=GomodIndent(v:lnum)

if exists("*GomodIndent")
  finish
endif

function! GomodIndent(lnum)
  let prevnum = prevnonblank(a:lnum-1)
  if prevnum == 0
    return 0
  endif

  " Take the current and previous lines, trimming end-of-line spaces
  " and comments.
  let line = substitute(getline(a:lnum), '\v\s+(//.*)?$', '', '')
  let prev = substitute(getline(prevnum), '\v\s+(//.*)?$', '', '')
  let ind = indent(prevnum)

  if prev =~ '\v^(require|replace|exclude).*\($'
    let ind += shiftwidth()
  endif

  if line =~ ')$'
    let ind -= shiftwidth()
  endif

  return ind
endfunction
