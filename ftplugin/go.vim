setlocal balloonexpr=GOVIMBalloonExpr()
setlocal omnifunc=GOVIMComplete
nnoremap <buffer> <silent> gd :GOVIMGoToDef<cr>
nnoremap <buffer> <silent> <C-]> :GOVIMGoToDef<cr>
nnoremap <buffer> <silent> <C-LeftMouse> <LeftMouse>:GOVIMGoToDef<cr>
nnoremap <buffer> <silent> g<LeftMouse> <LeftMouse>:GOVIMGoToDef<cr>
nnoremap <buffer> <silent> <C-t> :GOVIMGoToPrevDef<cr>
nnoremap <buffer> <silent> <C-RightMouse> <RightMouse>:GOVIMGoToPrevDef<cr>
nnoremap <buffer> <silent> g<RightMouse> <RightMouse>:GOVIMGoToPrevDef<cr>
