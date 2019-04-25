" This file represents the minimal .vimrc needed to work with govim
" It is used as part of the automated tests for govim and so will
" always be current

set nocompatible
set nobackup
set nowritebackup
set noswapfile

filetype plugin on

set mouse=a

" To get hover working in the terminal we need to set ttymouse. See
"
" :help ttymouse
"
" for the appropriate setting for your terminal. Note that despite the
" automated tests using xterm as the terminal, a setting of ttymouse=xterm
" does not work correctly beyond a certain column number (citation needed)
" hence we use ttymouse=sgr
set ttymouse=sgr
