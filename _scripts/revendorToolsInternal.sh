#!/usr/bin/env vbash

source "${BASH_SOURCE%/*}/common.bash"

shopt -s globstar
shopt -s extglob

go mod download

tools=$(go list -m -f={{.Dir}} golang.org/x/tools)

echo "Tools is $tools"

cd $(git rev-parse --show-toplevel)
rsync -a --delete --chmod=D0755,F0644 $tools/internal/ ./cmd/govim/internal/golang_org_x_tools
sed -i 's+golang.org/x/tools/internal+github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools+g' ./cmd/govim/internal/golang_org_x_tools/**/*.go
rm ./cmd/govim/internal/golang_org_x_tools/**/*_test.go
rm -f ./cmd/govim/internal/golang_org_x_tools/LICENSE
cp $tools/LICENSE ./cmd/govim/internal/golang_org_x_tools

go mod tidy
