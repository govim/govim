package nodisk

import (
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/foo"
)

func _() {
	foo.Foo() //@complete("F", Foo, IntFoo, StructFoo)
}
