#!/usr/bin/env vbash

go version
vim --version

./_scripts/revendorToolsInternal.sh

go install golang.org/x/tools/cmd/gopls

go generate ./...
go test ./...
go vet $(go list ./... | grep -v 'govim/internal')
gobin -m -run honnef.co/go/tools/cmd/staticcheck $(go list ./... | grep -v 'govim/internal')

go mod tidy
# https://github.com/golang/go/issues/27868#issuecomment-431413621
go list all > /dev/null

diff <(echo -n) <(gofmt -d $(git ls-files '**/*.go' '*.go' | grep -v cmd/govim/internal))
test -z "$(git status --porcelain)" || (git status; git diff; false)
