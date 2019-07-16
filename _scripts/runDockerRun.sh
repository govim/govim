#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"

doBranchCheck

cd "${BASH_SOURCE%/*}/../"

proxy=""

if [ "${CI:-}" != "true" ]
then
	proxy="-v $GOPATH/pkg/mod/cache/download:/cache -e GOPROXY=file:///cache"
fi

docker run $proxy --env-file ./_scripts/.docker_env_file -e "VIM_FLAVOR=${VIM_FLAVOR:-vim}" -v $PWD:/home/$USER/govim -w /home/$USER/govim --rm govim ./_scripts/dockerRun.sh
