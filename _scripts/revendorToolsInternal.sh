#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"

shopt -s extglob

# Save location of root of repo and change there to start
SCRIPT_DIR="$( command cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"/..
cd $SCRIPT_DIR

# Ensure that all dependencies are downloaded and present
# in the module cache
go mod download

# Get module cache directories of gopls and tools.
# Note this gives us the result post replace directives
# which is what we want.
tools=$(go list -m -f={{.Dir}} golang.org/x/tools)
gopls=$(go list -m -f={{.Dir}} golang.org/x/tools/gopls)

# Establish a temporary module to collect and vendor
# our internal requirements
td=$(mktemp -d)
trap "rm -rf $td" EXIT
cd $td
go mod init example.com
cat <<EOD > deps.go
package deps

import (
	_ "golang.org/x/tools/gopls/internal/lsp/protocol"
	_ "golang.org/x/tools/gopls/internal/lsp/source"
	_ "golang.org/x/tools/gopls/internal/lsp/command"
	_ "golang.org/x/tools/gopls/internal/span"
	_ "golang.org/x/tools/internal/fakenet"
	_ "golang.org/x/tools/internal/jsonrpc2"
)
EOD

# Add replace directives by hand because go mod edit
# misinterprets a directory containing an '@' as an
# indication of the version
cat <<EOD >> go.mod
replace golang.org/x/tools => $tools
replace golang.org/x/tools/gopls => $gopls
EOD
go mod tidy
go mod vendor

cd $SCRIPT_DIR

tools_regex='s+golang.org/x/tools/internal+github.com/govim/govim/cmd/govim/internal/golang_org_x_tools+g'
gopls_regex='s+golang.org/x/tools/gopls/internal+github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls+g'

if [ $(go env GOHOSTOS) = 'darwin' ]; then
    rsync -a --delete --chmod=Du+w,Fu+w $td/vendor/golang.org/x/tools/internal/ ./cmd/govim/internal/golang_org_x_tools
    rsync -a --delete --chmod=Du+w,Fu+w $td/vendor/golang.org/x/tools/gopls/internal/ ./cmd/govim/internal/golang_org_x_tools_gopls
    find ./cmd/govim/internal/golang_org_x_tools ./cmd/govim/internal/golang_org_x_tools_gopls  -name "*.go" -exec sed -i '' -e $tools_regex {} +
    find ./cmd/govim/internal/golang_org_x_tools_gopls -name "*.go" -exec sed -i '' -e $gopls_regex {} +
else
    rsync -a --delete --chmod=D0755,F0644 $td/vendor/golang.org/x/tools/internal/ ./cmd/govim/internal/golang_org_x_tools
    rsync -a --delete --chmod=D0755,F0644 $td/vendor/golang.org/x/tools/gopls/internal/ ./cmd/govim/internal/golang_org_x_tools_gopls
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
