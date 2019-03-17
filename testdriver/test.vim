set nocompatible
set nobackup
set nowritebackup
set noswapfile

" Useful for debugging
" call ch_logfile("/tmp/vimchannel.out", "a")
" set verbosefile=/tmp/vim.out
" set verbose=9

" TODO workout how to make this OS agnostic in terms of slash
let filename = expand(expand("<sfile>:h:h"))."/plugin/govim.vim"
execute 'source '.filename
