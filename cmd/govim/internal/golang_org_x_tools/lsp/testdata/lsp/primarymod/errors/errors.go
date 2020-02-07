package errors

import (
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/types"
)

func _() {
	bob.Bob() //@complete(".")
	types.b //@complete(" //", Bob_interface)
}
