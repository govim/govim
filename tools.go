// +build tools

package tools

import (
	_ "github.com/myitcv/vbash"
	_ "golang.org/x/tools/cmd/gopls"
	_ "golang.org/x/tools/cmd/stringer"
	_ "honnef.co/go/tools/cmd/staticcheck"
)
