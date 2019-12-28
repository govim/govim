#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"

doBranchCheck

cd "${BASH_SOURCE%/*}/../"

proxy=""

artefacts="$HOME/artefacts"

if [ "${CI:-}" != "true" ]
then
	go mod download
	modcache="$(go env GOPATH | sed -e 's/:/\n/' | head -n 1)/pkg/mod/cache/download"
	proxy="-v $modcache:/cache -e GOPROXY=file:///cache"
	artefacts="$(mktemp -d)"
fi

mkdir -p $artefacts

docker run $proxy --env-file ./_scripts/.docker_env_file -e "VIM_FLAVOR=${VIM_FLAVOR:-vim}" -v $artefacts:/artefacts -e GOTMPDIR=/artefacts -v $PWD:/home/$USER/govim -w /home/$USER/govim --rm govim ./_scripts/dockerRun.sh
