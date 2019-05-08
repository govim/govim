#!/usr/bin/env vbash

set -euo pipefail

cd "${BASH_SOURCE%/*}"

# Usage; either:
#
#   buildGovimImage.sh
#   buildGovimImage.sh VIMFLAVOR VIMVERSION GOVERSION
#

if [ "$#" -eq 3 ]
then
    VIM_FLAVOR="$1"
	VIM_VERSION="$2"
	GO_VERSION="$3"
else
    VIM_FLAVOR="${VIM_FLAVOR:-vim}"
	VIM_VERSION=$(echo $VIM_VERSIONS | tr ',' '\n' | tail -n 1)
	GO_VERSION=$(echo $GO_VERSIONS | tr ',' '\n' | tail -n 1)
fi


cat Dockerfile.user \
    | GO_VERSION=$GO_VERSION VIM_FLAVOR=$VIM_FLAVOR VERSION=$VIM_VERSION envsubst '$GO_VERSION,$VIM_FLAVOR,$VERSION' \
    | docker build -t govim --build-arg USER=$USER --build-arg UID=$UID --build-arg GID=$(id -g $USER) -
