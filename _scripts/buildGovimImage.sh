#!/usr/bin/env vbash

set -eu

# Usage; either:
#
#   buildGovimImage.sh
#   buildGovimImage.sh VIMVERSION GOVERSION
#

if [ "$#" -eq 2 ]
then
	VIM_VERSION="$1"
	GO_VERSION="$2"
else
	VIM_VERSION=$(echo $VIM_VERSIONS | tr ',' '\n' | tail -n 1)
	GO_VERSION=$(echo $GO_VERSIONS | tr ',' '\n' | tail -n 1)
fi

cat Dockerfile.user | GO_VERSION=$GO_VERSION VIM_VERSION=$VIM_VERSION envsubst '$GO_VERSION,$VIM_VERSION' | docker build -t govim --build-arg USER=$USER --build-arg UID=$UID --build-arg GID=$(id -g $USER) -
