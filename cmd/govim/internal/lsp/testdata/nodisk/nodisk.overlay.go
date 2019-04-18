package nodisk

import (
	"github.com/myitcv/govim/cmd/govim/internal/lsp/foo"
)

func _() {
	foo.Foo() //@complete("F", Foo, IntFoo, StructFoo)
}
