## `govim` - Go development plugin for Vim8

Command `github.com/myitcv/govim/cmd/govim` (referred to simply as `govim`) is a Go development plugin for Vim8, much
like [`vim-go`](https://github.com/fatih/vim-go). But unlike `vim-go`, `govim` is written in Go, not VimScript. It has
features like code completion, format-on-save, hover details and go-to definition, all of which are driven by
[`gopls`](https://godoc.org/golang.org/x/tools/cmd/gopls), the Language Server Protocol (LSP) server for Go. See [the
wiki](https://github.com/myitcv/govim/wiki/govim-plugin-API) for more details. Installation instructions below.

Package [`github.com/myitcv/govim`](https://godoc.org/github.com/myitcv/govim) provides an API for plugin developers to
interface with Vim8 in Go. More details [here](PLUGIN_AUTHORS.md).

`govim` requires at least [`go1.12`](https://golang.org/dl/) and [Vim `v8.1.1158`](https://www.vim.org/download.php)
(`gvim` is also supported). [Neovim](https://neovim.io) is not (currently) supported. More details [in the
FAQ](https://github.com/myitcv/govim/wiki/FAQ#what-versions-of-vim-and-go-are-supported-with-govim).

Install `govim` via:

* [Vim 8 packages](http://vimhelp.appspot.com/repeat.txt.html#packages)
  * `git clone https://github.com/myitcv/govim.git ~/.vim/pack/plugins/start/govim`
* [Pathogen](https://github.com/tpope/vim-pathogen)
  * `git clone https://github.com/myitcv/govim.git ~/.vim/bundle/govim`
* [vim-plug](https://github.com/junegunn/vim-plug)
  * `Plug 'myitcv/govim'`
* [Vundle](https://github.com/VundleVim/Vundle.vim)
  * `Plugin 'myitcv/govim'`

You might need some `.vimrc` settings to get all features working: see the [minimal `.vimrc`](minimal.vimrc) for an
commented explanation of the required settings. For more details on `.vimrc`/`.gvimrc` settings as well as some tips and
tricks, see [here](https://github.com/myitcv/govim/wiki/vimrc-tips).

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
