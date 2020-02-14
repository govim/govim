package danglingstmt

import "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/foo"

func _() {
	foo. //@rank(" //", Foo)
	var _ = []string{foo.} //@rank("}", Foo)
}
