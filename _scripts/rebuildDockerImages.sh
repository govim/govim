#!/usr/bin/env vbash

set -euo pipefail

cd "${BASH_SOURCE%/*}"

# Usage; either:
#
#   rebuildDockerImages.sh
#   rebuildDockerImages.sh VIMVERSION GOVERSION NEOVIMVERSION
#

if [ "$#" -eq 3 ]
then
	VIM_VERSIONS="$1"
	GO_VERSIONS="$2"
	NEOVIM_VERSIONS="$3"
else
	if [ "${VIM_VERSIONS:-}" == "" ]
	then
		VIM_VERSIONS="$VIM_VERSION"
	fi
	if [ "${GO_VERSIONS:-}" == "" ]
	then
		GO_VERSIONS="$GO_VERSION"
	fi
	if [ "${NEOVIM_VERSIONS:-}" == "" ]
	then
		NEOVIM_VERSIONS="$NEOVIM_VERSION"
	fi
fi

vbashVersion="$(go list -m -f={{.Version}} github.com/myitcv/vbash)"

# Build base layer image
docker build --build-arg "GH_USER=$GH_USER" --build-arg "GH_TOKEN=$GH_TOKEN" --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=${GO_VERSION}" -t govim/govim:base_${GO_VERSION}_${vbashVersion} -f Dockerfile.base .


# Build Vim images
for i in $(echo "$VIM_VERSIONS" | tr ',' '\n')
do
	for j in $(echo "$GO_VERSIONS" | tr ',' '\n')
	do
		docker build --build-arg "GH_USER=$GH_USER" --build-arg "GH_TOKEN=$GH_TOKEN" --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=$j" --build-arg "VIM_VERSION=$i" -t govim/govim:${j}_vim_${i}_v1 -f Dockerfile.vim .
	done
done


# Build Neovim images
for i in $(echo "$NEOVIM_VERSIONS" | tr ',' '\n')
do
	for j in $(echo "$GO_VERSIONS" | tr ',' '\n')
	do
		docker build --build-arg "GH_USER=$GH_USER" --build-arg "GH_TOKEN=$GH_TOKEN" --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=$j" --build-arg "NEOVIM_VERSION=$i" -t govim/govim:${j}_nvim_${i}_v1 -f Dockerfile.nvim .
	done
done
