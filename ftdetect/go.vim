" For now, rely on the .go detection, syntax, indent, etc. provided in recent
" Vim distributions.

" By default, vim associates .mod files with filetypes lprolog or modsim3.
" Override these rather than using setfiletype.
autocmd BufNewFile,BufRead *.mod setlocal filetype=gomod
