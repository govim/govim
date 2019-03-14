## `github.com/myitcv/govim/cmd/govim`

`govim` is a Vim8 channel-based plugin, written in Go, to support the writing of Go code in Vim. WIP; still lots
[TODO](https://github.com/myitcv/govim/wiki/TODO).

Instructions below use [`gobin`](https://github.com/myitcv/gobin):

```
gobin github.com/myitcv/govim/cmd/govim
vi -u run.vim
```

Then within Vim:

```
:echo Hello()
```

should result in:

```
World
```

If you are working on `github.com/myitcv/govim/cmd/govim` locally, then set the environment variable:

```
GOVIM_RUNCMD="gobin -m -run github.com/myitcv/govim/cmd/govim"
```

to run the main-module local version of `govim`.

Tested against Vim 8.1. Untested/unexpected to (currently) work with Neovim.

### Tests

See [`govim` command tests](https://github.com/myitcv/govim/wiki/govim-command-tests).

### TODO

See [TODO](https://github.com/myitcv/govim/wiki/TODO).
