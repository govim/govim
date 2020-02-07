package other

import "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/rename/crosspkg"

func Other() {
	crosspkg.Bar
	crosspkg.Foo() //@rename("Foo", "Flamingo")
}
