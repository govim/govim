package errors

import (
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/types"
)

func _() {
	bob.Bob() //@complete(".")
	types.b //@complete(" //", Bob_interface)
}
