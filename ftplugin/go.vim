if GOVIMPluginStatus() == "initcomplete"
  setlocal balloonexpr=GOVIM_internal_BalloonExpr()
  setlocal omnifunc=GOVIM_internal_Complete
  nnoremap <buffer> <silent> gd :GOVIMGoToDef<cr>
  nnoremap <buffer> <silent> <C-]> :GOVIMGoToDef<cr>
  nnoremap <buffer> <silent> <C-LeftMouse> <LeftMouse>:GOVIMGoToDef<cr>
  nnoremap <buffer> <silent> g<LeftMouse> <LeftMouse>:GOVIMGoToDef<cr>
  nnoremap <buffer> <silent> <C-t> :GOVIMGoToPrevDef<cr>
  nnoremap <buffer> <silent> <C-RightMouse> :GOVIMGoToPrevDef<cr>
  nnoremap <buffer> <silent> g<RightMouse> :GOVIMGoToPrevDef<cr>
endif
