package links

import (
	"fmt" //@link(re`".*"`,"https://godoc.org/fmt")

	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/foo" //@link(re`".*"`,`https://godoc.org/github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/foo`)
)

var (
	_ fmt.Formatter
	_ foo.StructFoo
)

// Foo function
func Foo() string {
	/*https://example.com/comment */ //@link("https://example.com/comment","https://example.com/comment")
	url := "https://example.com/string_literal" //@link("https://example.com/string_literal","https://example.com/string_literal")
	return url
}
