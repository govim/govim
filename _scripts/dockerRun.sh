#!/usr/bin/env vbash

source "${BASH_SOURCE%/*}/common.bash"

if [ "${VIM_COMMAND:-}" == "" ]
then
	eval "VIM_COMMAND=\"\$DEFAULT_${VIM_FLAVOR^^}_COMMAND\""
fi

cat <<EOD
Environment is:
  VIM_FLAVOR=$VIM_FLAVOR
  VIM_COMMAND=$VIM_COMMAND
EOD

if [ "${GH_USER:-}" != "" ] && [ "${GH_TOKEN:-}" != "" ]
then
	echo -e "machine api.github.com\n  login $GH_USER\n  password $GH_TOKEN" >> ~/.netrc
	echo -e "machine github.com\n  login $GH_USER\n  password $GH_TOKEN" >> ~/.netrc
	echo -e "machine githubusercontent.com\n  login $GH_USER\n  password $GH_TOKEN" >> ~/.netrc
fi

go version
vim --version

./_scripts/revendorToolsInternal.sh

go install golang.org/x/tools/gopls

# remove all generated files to ensure we are never stale
rm -f $(git ls-files -- ':!:cmd/govim/internal/golang_org_x_tools' '**/gen_*.*' 'gen_*.*') .travis.yml

go generate $(go list ./... | grep -v 'govim/internal/golang_org_x_tools')
go test $(go list ./... | grep -v 'govim/internal/golang_org_x_tools')

if [ "${CI:-}" == "true" ] && [ "${TRAVIS_BRANCH:-}_${TRAVIS_PULL_REQUEST_BRANCH:-}" == "master_" ]
then
	go test -race $(go list ./... | grep -v 'govim/internal/golang_org_x_tools')
fi

go vet $(go list ./... | grep -v 'govim/internal/golang_org_x_tools')
go run honnef.co/go/tools/cmd/staticcheck $(go list ./... | grep -v 'govim/internal/golang_org_x_tools')

if [ "${CI:-}" == "true" ]
then
	go mod tidy
	# https://github.com/golang/go/issues/27868#issuecomment-431413621
	go list all > /dev/null

	diff <(echo -n) <(gofmt -d $(git ls-files '**/*.go' '*.go' | grep -v cmd/govim/internal/golang_org_x_tools))
	test -z "$(git status --porcelain)" || (git status; git diff; false)
fi
