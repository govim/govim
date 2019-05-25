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

for j in $GO_VERSIONS
do
	# Build base layer image
	docker build --build-arg "GH_USER=$GH_USER" --build-arg "GH_TOKEN=$GH_TOKEN" --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=${j}" -t govim/govim:base_${j}_${vbashVersion} -f Dockerfile.base .

	for ii in $VALID_FLAVORS
	do
		for i in $(eval "echo \$${ii^^}_VERSIONS")
		do
			docker build --build-arg "GH_USER=$GH_USER" --build-arg "GH_TOKEN=$GH_TOKEN" --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=$j" --build-arg "VIM_VERSION=$i" -t govim/govim:${j}_${ii}_${i}_v1 -f Dockerfile.${ii} . ##
		done
	done
done

