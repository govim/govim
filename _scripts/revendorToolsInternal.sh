#!/usr/bin/env vbash

shopt -s globstar
shopt -s extglob

go mod download

tools=$(go list -m -f={{.Dir}} golang.org/x/tools)

echo "Tools is $tools"

cd $(git rev-parse --show-toplevel)
rsync -a --delete --chmod=D0755,F0644 $tools/internal/ ./cmd/govim/internal/
sed -i 's+golang.org/x/tools/internal+github.com/myitcv/govim/cmd/govim/internal+g' ./cmd/govim/internal/**/*.go
rm ./cmd/govim/internal/**/*_test.go
rm -f ./cmd/govim/internal/LICENSE
cp $tools/LICENSE ./cmd/govim/internal/

go mod tidy
