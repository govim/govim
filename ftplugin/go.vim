" Hover
setlocal balloonexpr=GOVIM_internal_BalloonExpr()

" Completion
setlocal omnifunc=GOVIM_internal_Complete

" go-to-def
nnoremap <buffer> <silent> gd :GOVIMGoToDef<cr>
nnoremap <buffer> <silent> <C-]> :GOVIMGoToDef<cr>
nnoremap <buffer> <silent> <C-LeftMouse> <LeftMouse>:GOVIMGoToDef<cr>
nnoremap <buffer> <silent> g<LeftMouse> <LeftMouse>:GOVIMGoToDef<cr>
nnoremap <buffer> <silent> <C-t> :GOVIMGoToPrevDef<cr>
nnoremap <buffer> <silent> <C-RightMouse> :GOVIMGoToPrevDef<cr>
nnoremap <buffer> <silent> g<RightMouse> :GOVIMGoToPrevDef<cr>

" Motions
nnoremap <buffer> <silent> [[ :call GOVIMMotion("prev", "File.Decls.Pos()")<cr>
nnoremap <buffer> <silent> [] :call GOVIMMotion("prev", "File.Decls.End()")<cr>
nnoremap <buffer> <silent> ][ :call GOVIMMotion("next", "File.Decls.Pos()")<cr>
nnoremap <buffer> <silent> ]] :call GOVIMMotion("next", "File.Decls.End()")<cr>
