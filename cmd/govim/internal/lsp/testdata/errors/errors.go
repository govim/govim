package errors

import (
	"github.com/myitcv/govim/cmd/govim/internal/lsp/types"
)

func _() {
	bob.Bob() //@complete(".")
	types.b //@complete(" //", Bob_interface)
}
