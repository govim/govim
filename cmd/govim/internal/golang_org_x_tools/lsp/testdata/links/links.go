package links

import (
	"fmt" //@link(`fmt`,"https://pkg.go.dev/fmt")

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/foo" //@link(`github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/foo`,`https://pkg.go.dev/github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/foo`)

	_ "database/sql" //@link(`database/sql`, `https://pkg.go.dev/database/sql`)

	errors "golang.org/x/xerrors" //@link(`golang.org/x/xerrors`, `https://pkg.go.dev/golang.org/x/xerrors`)
)

var (
	_ fmt.Formatter
	_ foo.StructFoo
	_ errors.Formatter
)

// Foo function
func Foo() string {
	/*https://example.com/comment */ //@link("https://example.com/comment","https://example.com/comment")

	url := "https://example.com/string_literal" //@link("https://example.com/string_literal","https://example.com/string_literal")
	return url
}
