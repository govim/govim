#!/usr/bin/env vbash

source "${BASH_SOURCE%/*}/common.bash"

shopt -s extglob

go mod download

tools=$(go list -m -f={{.Dir}} golang.org/x/tools)
gopls=$(go list -m -f={{.Dir}} golang.org/x/tools/gopls)

echo "Tools is $tools, gopls is $gopls"

cd $(git rev-parse --show-toplevel)

tools_regex='s+golang.org/x/tools/internal+github.com/govim/govim/cmd/govim/internal/golang_org_x_tools+g'
gopls_regex='s+golang.org/x/tools/gopls/internal+github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls+g'

if [ $(go env GOHOSTOS) = 'darwin' ]; then
    rsync -a --delete --chmod=Du+w,Fu+w $tools/internal/ ./cmd/govim/internal/golang_org_x_tools
    rsync -a --delete --chmod=Du+w,Fu+w $gopls/internal/ ./cmd/govim/internal/golang_org_x_tools_gopls
    find ./cmd/govim/internal/golang_org_x_tools ./cmd/govim/internal/golang_org_x_tools_gopls  -name "*.go" -exec sed -i '' -e $tools_regex {} +
    find ./cmd/govim/internal/golang_org_x_tools_gopls -name "*.go" -exec sed -i '' -e $gopls_regex {} +
else
    rsync -a --delete --chmod=D0755,F0644 $tools/internal/ ./cmd/govim/internal/golang_org_x_tools
    rsync -a --delete --chmod=D0755,F0644 $gopls/internal/ ./cmd/govim/internal/golang_org_x_tools_gopls
    find ./cmd/govim/internal/golang_org_x_tools ./cmd/govim/internal/golang_org_x_tools_gopls -name "*.go" -exec sed -i $tools_regex {} +
    find ./cmd/govim/internal/golang_org_x_tools_gopls -name "*.go" -exec sed -i $gopls_regex {} +
fi

# Remove _test.go files and testdata directories
find ./cmd/govim/internal/golang_org_x_tools/ ./cmd/govim/internal/golang_org_x_tools_gopls -name "*_test.go" -exec rm {} +
find ./cmd/govim/internal/golang_org_x_tools/ ./cmd/govim/internal/golang_org_x_tools_gopls -type d -name testdata -exec rm -rf {} +

# Copy license
cp $tools/LICENSE ./cmd/govim/internal/golang_org_x_tools
cp $gopls/LICENSE ./cmd/govim/internal/golang_org_x_tools_gopls

# Hack to work around golang.org/issue/40067
if go list -f {{context.ReleaseTags}} runtime | grep $(echo "$MAX_GO_VERSION" | sed -e 's/^\([^.]*\.[^.]*\).*/\1/') > /dev/null
then
	go mod tidy
fi
