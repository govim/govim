#!/usr/bin/env vbash

set -u

source "${BASH_SOURCE%/*}/gen_maxVersions_genconfig.bash"

validFlavor=0
for i in $VALID_FLAVORS
do
	if [ "${VIM_FLAVOR:-}" == "$i" ]
	then
		validFlavor=1
	fi
done

if [ $validFlavor -ne 1 ]
then
	echo "VIM_FLAVOR is not set to a valid value; must be one of: $VALID_FLAVORS"
	exit 1
fi

if [ "${VIM_COMMAND:-}" == "" ]
then
	case "$VIM_FLAVOR" in
		vim)
			export VIM_COMMAND=$DEFAULT_VIM_COMMAND
			;;
		gvim)
			export VIM_COMMAND=$DEFAULT_GVIM_COMMAND
			;;
	esac
fi

cat <<EOD
Environment is:
  VIM_FLAVOR=$VIM_FLAVOR
  VIM_COMMAND=$VIM_COMMAND
EOD

echo -e "machine github.com\n  login $GH_USER\n  password $GH_TOKEN" >> ~/.netrc
echo -e "machine githubusercontent.com\n  login $GH_USER\n  password $GH_TOKEN" >> ~/.netrc

go version
vim --version

./_scripts/revendorToolsInternal.sh

go install golang.org/x/tools/cmd/gopls

# remove all generated files to ensure we are never stale
rm -f $(git ls-files -- ':!:cmd/govim/internal' '**/gen_*.*' 'gen_*.*') .travis.yml

go generate $(go list ./... | grep -v 'govim/internal')
go test $(go list ./... | grep -v 'govim/internal')

if [ "${CI:-}" == "true" ]
then
	go test -race $(go list ./... | grep -v 'govim/internal')
fi

go vet $(go list ./... | grep -v 'govim/internal')
go run honnef.co/go/tools/cmd/staticcheck $(go list ./... | grep -v 'govim/internal')

if [ "${CI:-}" == "true" ]
then
	go mod tidy
	# https://github.com/golang/go/issues/27868#issuecomment-431413621
	go list all > /dev/null

	diff <(echo -n) <(gofmt -d $(git ls-files '**/*.go' '*.go' | grep -v cmd/govim/internal))
	test -z "$(git status --porcelain)" || (git status; git diff; false)
fi
