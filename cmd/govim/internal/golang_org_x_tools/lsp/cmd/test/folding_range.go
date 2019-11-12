package cmdtest

import (
	"fmt"
	"testing"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/cmd"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/tool"
)

func (r *runner) FoldingRanges(t *testing.T, spn span.Span) {
	goldenTag := "foldingRange-cmd"
	uri := spn.URI()
	filename := uri.Filename()

	app := cmd.New("gopls-test", r.data.Config.Dir, r.data.Config.Env, r.options)
	got := CaptureStdOut(t, func() {
		err := tool.Run(r.ctx, app, append([]string{"-remote=internal", "folding_ranges"}, filename))
		if err != nil {
			fmt.Println(err)
		}
	})

	expect := string(r.data.Golden(goldenTag, filename, func() ([]byte, error) {
		return []byte(got), nil
	}))

	if expect != got {
		t.Errorf("folding_ranges failed failed for %s expected:\n%s\ngot:\n%s", filename, expect, got)
	}
}
