#!/usr/bin/env vbash

echo -e "machine github.com\n  login $GH_USER\n  password $GH_TOKEN" >> ~/.netrc
echo -e "machine githubusercontent.com\n  login $GH_USER\n  password $GH_TOKEN" >> ~/.netrc

go version
vim --version

./_scripts/revendorToolsInternal.sh

go install golang.org/x/tools/cmd/gopls

# remove all generated files to ensure we are never stale
rm -f $(git ls-files -- ':!:cmd/govim/internal' '**/gen_*.go' 'gen_*.go')

go generate ./...
go test ./...
go test -race ./...
go vet $(go list ./... | grep -v 'govim/internal')
go run honnef.co/go/tools/cmd/staticcheck $(go list ./... | grep -v 'govim/internal')

go mod tidy
# https://github.com/golang/go/issues/27868#issuecomment-431413621
go list all > /dev/null

diff <(echo -n) <(gofmt -d $(git ls-files '**/*.go' '*.go' | grep -v cmd/govim/internal))
test -z "$(git status --porcelain)" || (git status; git diff; false)
