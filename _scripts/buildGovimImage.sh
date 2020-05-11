#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"

doBranchCheck

# Usage; either:
#
#   buildGovimImage.sh
#   buildGovimImage.sh VIM_FLAVOR VIM_VERSION GO_VERSION
#
# Note that VIM_FLAVOR can be one of vim or gvim and VIM_VERSION is a version
# pertaining to any of them.

cd "${BASH_SOURCE%/*}"

if [ "$#" -eq 3 ]
then
	VIM_FLAVOR="$1"
	VIM_VERSION="$2"
	GO_VERSION="$3"
else
	# If not provided we default to testing against vim.
	VIM_FLAVOR="${VIM_FLAVOR:-vim}"
	if [ "${VIM_VERSION:-}" == "" ]
	then
		eval "VIM_VERSION=\"\$MAX_${VIM_FLAVOR^^}_VERSION\""
	fi
	if [ "${GO_VERSION:-}" == "" ]
	then
		GO_VERSION="$MAX_GO_VERSION"
	fi
fi

cat Dockerfile.user \
	| GO_VERSION=$GO_VERSION VIM_FLAVOR=$VIM_FLAVOR VIM_VERSION=$VIM_VERSION envsubst '$GO_VERSION,$VIM_FLAVOR,$VIM_VERSION' \
	| docker build --progress plain -t govim --build-arg USER=$USER --build-arg UID=$UID --build-arg GID=$(id -g $USER) -
