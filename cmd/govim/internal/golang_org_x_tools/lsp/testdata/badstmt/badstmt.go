package badstmt

import (
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/foo"
)

func _() {
	defer foo.F //@complete("F", Foo, IntFoo, StructFoo),diag(" //", "LSP", "function must be invoked in defer statement")
	go foo.F //@complete("F", Foo, IntFoo, StructFoo)
}