set nocompatible
set nobackup
set nowritebackup
set noswapfile

" TODO workout how to make this OS agnostic in terms of slash
let filename = expand(expand("<sfile>:h:h"))."/plugin/govim.vim"
execute 'source '.filename
