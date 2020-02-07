package b

import (
	"fmt"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/godef/a"
)

func useThings() {
	t := a.Thing{}      //@mark(bStructType, "ing")
	fmt.Print(t.Member) //@mark(bMember, "ember")
	fmt.Print(a.Other)  //@mark(bVar, "ther")
	a.Things()          //@mark(bFunc, "ings")
}

/*@
godef(bStructType, Thing)
godef(bMember, Member)
godef(bVar, Other)
godef(bFunc, Things)
*/
