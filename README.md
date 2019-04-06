## `govim` - Go-based Vim8 plugins

Command `github.com/myitcv/govim/cmd/govim` (referred to simply as `govim`) is a Go development plugin for Vim8, much
like [`vim-go`](https://github.com/fatih/vim-go). Installation instructions are below.

Package [`github.com/myitcv/govim`](https://godoc.org/github.com/myitcv/govim) provides an API for plugin developers to
interface with Vim8 in Go. More details [here](PLUGIN_AUTHORS.md).

`govim` requires at least [`go1.12`](https://golang.org/dl/) and [Vim `v8.1.0053`](https://www.vim.org/download.php)
(`gvim` is also supported). More details [in the
FAQ](https://github.com/myitcv/govim/wiki/FAQ#what-versions-of-vim-and-go-are-supported).

Install `govim` via:

* [Vim 8 packages](http://vimhelp.appspot.com/repeat.txt.html#packages)
  * `git clone https://github.com/myitcv/govim.git ~/.vim/pack/plugins/start/govim`
* [Pathogen](https://github.com/tpope/vim-pathogen)
  * `git clone https://github.com/myitcv/govim.git ~/.vim/bundle/govim`
* [vim-plug](https://github.com/junegunn/vim-plug)
  * `Plug 'myitcv/govim'`
* [Vundle](https://github.com/VundleVim/Vundle.vim)
  * `Plugin 'myitcv/govim'`

### What can `govim` do?

See [`govim` plugin API](https://github.com/myitcv/govim/wiki/govim-plugin-API) which also has links to some demo
screencasts.

### FAQ

Top of your list of questions is likely _"Why have you created govim? What is/was wrong with `vim-go`?"_ For answers
this and more see [FAQ](https://github.com/myitcv/govim/wiki/FAQ).

### Contributing

Contributions are very much welcome in the form of:

* feedback
* issues
* PRs

See [`govim` tests](https://github.com/myitcv/govim/wiki/govim-tests) for details on how the modules in this repository
are tested.
