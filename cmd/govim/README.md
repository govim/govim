## `github.com/myitcv/govim/cmd/govim`

These instructions are just temporary:

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

If you are working on `github.com/myitcv/govim/cmd/govim` locally, then set:

```
GOVIM_RUNCMD="gobin -m -run github.com/myitcv/govim/cmd/govim"
```

to run the main-module local version.

Tested against Vim 8.1. Untested/unexpected to (currently) work with Neovim.

### Tests

Tests are written using [`testscript`](https://godoc.org/github.com/rogpeppe/go-internal/testscript) scripts. For each
script, an instance of Vim is started with the `govim` plugin. The Vim instance can be controlled using the `vim`
command. For example:

```bash
# get the line number of the last line
vim call line '["$"]'

# evaluate expressions
vim expr '[1, 2, line("$")]'

# run ex commands
vim ex 'w test'

# run normal commands
vim normal h

# cause vim to redraw
vim redraw
```

See the scripts in [`testdata`](testdata) for more ideas/examples.

### TODO

* Implement support for defining:
  * range-based functions
  * commands
  * auto-cmds
* More tests
