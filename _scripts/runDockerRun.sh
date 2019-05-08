#!/usr/bin/env bash

set -euo pipefail

cd "${BASH_SOURCE%/*}/../"

proxy=""

if [ "${CI:-}" != "true" ]
then
	proxy="-v $GOPATH/pkg/mod/cache/download:/cache -e GOPROXY=file:///cache"
fi

if [ "${VIM_COMMAND:-}" == "" ]
then
	vimCmd="vim"
else
	vimCmd="$VIM_COMMAND"
fi

docker run $proxy --env-file ./_scripts/.docker_env_file -e "VIM_COMMAND=$vimCmd" -v $PWD:/home/$USER/govim -w /home/$USER/govim --rm govim ./_scripts/dockerRun.sh
