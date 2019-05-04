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
	GO_VERSIONS="$2"
else
	if [ "${VIM_VERSIONS:-}" == "" ]
	then
		VIM_VERSIONS="$VIM_VERSION"
	fi
	if [ "${GO_VERSIONS:-}" == "" ]
	then
		GO_VERSIONS="$GO_VERSION"
	fi
fi

vbashVersion="$(go list -m -f={{.Version}} github.com/myitcv/vbash)"

for i in $(echo "$VIM_VERSIONS" | tr ',' '\n')
do
	for j in $(echo "$GO_VERSIONS" | tr ',' '\n')
	do
		echo running docker build --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=$j" --build-arg "VIM_VERSION=$i" -t govim/govim:${j}_${i} .
		docker build --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=$j" --build-arg "VIM_VERSION=$i" -t govim/govim:${j}_${i}_v1 .
	done
done
