package b

import (
	_ "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/circular/double/one" //@diag("_ \"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/circular/double/one\"", "go list", "import cycle not allowed")
)
