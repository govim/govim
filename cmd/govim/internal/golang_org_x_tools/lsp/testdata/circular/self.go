package circular //@diag("", "go list", "import cycle not allowed")

import (
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/circular"
)

func print() {
	Test()
}

func Test() {
}
