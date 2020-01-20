#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"

doBranchCheck

cd "${BASH_SOURCE%/*}/../"

# The ARTEFACTS variable set by .travis.yml cannot expand
# variables so we do that here
ARTEFACTS=$(echo ${ARTEFACTS:-})

artefacts=""
proxy=""

if [ "${CI:-}" != "true" ]
then
	go mod download
	modcache="$(go env GOPATH | sed -e 's/:/\n/' | head -n 1)/pkg/mod/cache/download"
	proxy="-v $modcache:/cache -e GOPROXY=file:///cache"
else
	mkdir -p $ARTEFACTS
	artefacts="-v $ARTEFACTS:/artefacts -e GOVIM_TESTSCRIPT_WORKDIR_ROOT=/artefacts"
fi

docker run $proxy --env-file ./_scripts/.docker_env_file -e "VIM_FLAVOR=${VIM_FLAVOR:-vim}" $artefacts -v $PWD:/home/$USER/govim -w /home/$USER/govim --rm govim ./_scripts/dockerRun.sh
