package a

import (
	_ "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/circular/triple/b" //@diag("_ \"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/circular/triple/b\"", "go list", "import cycle not allowed")
)
