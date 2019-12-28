#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"

cd $HOME

if [ ! -d ./artefacts ]
then
	exit
fi

sudo find ./artefacts \( -name .vim -o -name gopath \) -prune -exec rm -rf '{}' \;
echo "=================================================================================="
tar -zc ./artefacts | base64
echo "=================================================================================="
