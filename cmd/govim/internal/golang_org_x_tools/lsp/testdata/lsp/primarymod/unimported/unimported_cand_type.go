package unimported

import (
	_ "context"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/baz"
	_ "github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/signature" // provide type information for unimported completions in the other file
)

func _() {
	foo.StructFoo{} //@item(litFooStructFoo, "foo.StructFoo{}", "struct{...}", "struct")

	// We get the literal completion for "foo.StructFoo{}" even though we haven't
	// imported "foo" yet.
	baz.FooStruct = f //@snippet(" //", litFooStructFoo, "foo.StructFoo{$0\\}", "foo.StructFoo{$0\\}")
}
