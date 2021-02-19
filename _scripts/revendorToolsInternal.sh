#!/usr/bin/env vbash

source "${BASH_SOURCE%/*}/common.bash"

shopt -s extglob

go mod download

tools=$(go list -m -f={{.Dir}} golang.org/x/tools)

echo "Tools is $tools"

cd $(git rev-parse --show-toplevel)
regex='s+golang.org/x/tools/internal+github.com/govim/govim/cmd/govim/internal/golang_org_x_tools+g'

if [ $(go env GOHOSTOS) = 'darwin' ]; then
    rsync -a --delete --chmod=Du+w,Fu+w $tools/internal/ ./cmd/govim/internal/golang_org_x_tools
    find ./cmd/govim/internal/golang_org_x_tools -name "*.go" -exec sed -i '' -e $regex {} +
else
    rsync -a --delete --chmod=D0755,F0644 $tools/internal/ ./cmd/govim/internal/golang_org_x_tools
    find ./cmd/govim/internal/golang_org_x_tools -name "*.go" -exec sed -i $regex {} +
fi

# Remove _test.go files and testdata directories
find ./cmd/govim/internal/golang_org_x_tools/ -name "*_test.go" -exec rm {} +
find ./cmd/govim/internal/golang_org_x_tools/ -type d -name testdata -exec rm -rf {} +

# Copy license
cp $tools/LICENSE ./cmd/govim/internal/golang_org_x_tools

# Hack to work around golang.org/issue/40067
if go list -f {{context.ReleaseTags}} runtime | grep $(echo "$MAX_GO_VERSION" | sed -e 's/^\([^.]*\.[^.]*\).*/\1/') > /dev/null
then
	go mod tidy
fi
