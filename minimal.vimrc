" This file represents the minimal .vimrc needed to work with govim
" It is used as part of the automated tests for govim and so will
" always be current

set nocompatible
set nobackup
set nowritebackup
set noswapfile

set mouse=a

" To get hover working in the terminal we need to set ttymouse. See
"
" :help ttymouse
"
" for the appropriate setting for your terminal.
set ttymouse=xterm2
