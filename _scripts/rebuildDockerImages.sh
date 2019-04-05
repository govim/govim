#!/usr/bin/env vbash

set -eu


# Usage; either:
#
#   rebuildDockerImsages.sh
#   rebuildDockerImsages.sh VIMVERSION GOVERSION
#

if [ "$#" -eq 2 ]
then
	VIM_VERSIONS="$1"
	GO_VERSION="$2"
else
	if [ "${VIM_VERSION:-}" != "" ]
	then
		VIM_VERSIONS="$VIM_VERSION"
	fi
	# $GO_VERSION used as is
fi

vbashVersion="$(go list -m -f={{.Version}} github.com/myitcv/vbash)"

for i in $(echo "$VIM_VERSIONS" | tr ',' '\n')
do
	echo docker build --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=$GO_VERSION" --build-arg "VIM_VERSION=$i" -t myitcv/govim:${GO_VERSION}_${i} .
	docker build --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=$GO_VERSION" --build-arg "VIM_VERSION=$i" -t myitcv/govim:${GO_VERSION}_${i} .
done
