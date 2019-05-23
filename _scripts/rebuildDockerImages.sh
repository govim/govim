#!/usr/bin/env vbash

set -euo pipefail

source "${BASH_SOURCE%/*}/gen_maxVersions_genconfig.bash"

cd "${BASH_SOURCE%/*}"

# Usage; either:
#
#   rebuildDockerImages.sh
#   rebuildDockerImages.sh VIM_VERSION GO_VERSION NEOVIM_VERSION
#

if [ "$#" -eq 3 ]
then
	VIM_VERSIONS="$1"
	GO_VERSIONS="$2"
	NEOVIM_VERSIONS="$3"
fi

vbashVersion="$(go list -m -f={{.Version}} github.com/myitcv/vbash)"

# Build base layer image
docker build --build-arg "GH_USER=$GH_USER" --build-arg "GH_TOKEN=$GH_TOKEN" --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=${GO_VERSION}" -t govim/govim:base_${GO_VERSION}_${vbashVersion} -f Dockerfile.base .

# TODO perhaps there is a better way to do this... probably in Go

for i in $VIM_VERSIONS
do
	for j in $GO_VERSIONS
	do
		docker build --build-arg "GH_USER=$GH_USER" --build-arg "GH_TOKEN=$GH_TOKEN" --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=$j" --build-arg "VIM_VERSION=$i" -t govim/govim:${j}_vim_${i}_v1 -f Dockerfile.vim . ##
	done
done

for i in $GVIM_VERSIONS
do
	for j in $GO_VERSIONS
	do
		docker build --build-arg "GH_USER=$GH_USER" --build-arg "GH_TOKEN=$GH_TOKEN" --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=$j" --build-arg "VIM_VERSION=$i" -t govim/govim:${j}_gvim_${i}_v1 -f Dockerfile.vim . ##
	done
done

# Hardcode for now
NEOVIM_VERSIONS=v0.3.5
for i in $NEOVIM_VERSIONS
do
	for j in $GO_VERSIONS
	do
		docker build --build-arg "GH_USER=$GH_USER" --build-arg "GH_TOKEN=$GH_TOKEN" --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=$j" --build-arg "VIM_VERSION=$i" -t govim/govim:${j}_nvim_${i}_v1 -f Dockerfile.nvim . ##
	done
done
