package links

import (
	"fmt" //@link(re`".*"`,"https://godoc.org/fmt")

	"github.com/myitcv/govim/cmd/govim/internal/lsp/foo" //@link(re`".*"`,"https://godoc.org/github.com/myitcv/govim/cmd/govim/internal/lsp/foo")
)

var (
	_ fmt.Formatter
	_ foo.StructFoo
)
