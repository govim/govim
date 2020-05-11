#!/usr/bin/env vbash

source "${BASH_SOURCE%/*}/common.bash"

cd "${BASH_SOURCE%/*}"

# Usage; either:
#
#   rebuildDockerImages.sh
#   rebuildDockerImages.sh VIM_VERSION GO_VERSION
#

if [ "$#" -eq 3 ]
then
	VIM_VERSIONS="$1"
	GO_VERSIONS="$2"
fi

vbashVersion="$(go list -m -f={{.Version}} github.com/myitcv/vbash)"

for j in $GO_VERSIONS
do
	for ii in $VALID_FLAVORS
	do
		for i in $(eval "echo \$${ii^^}_VERSIONS")
		do
			docker build --progress plain --build-arg "GH_USER=$GH_USER" --build-arg "GH_TOKEN=$GH_TOKEN" --build-arg "VBASHVERSION=$vbashVersion" --build-arg "GO_VERSION=$j" --build-arg "VIM_VERSION=$i" -t govim/govim:${j}_${ii}_${i}_v1 -f Dockerfile.${ii} . ##
		done
	done
done

