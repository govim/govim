#!/usr/bin/env bash

set -euo pipefail

source "${BASH_SOURCE%/*}/gen_maxVersions_genconfig.bash"

# trimLocalDockerImages trims down your local Docker image cache to contain
# just the matrix implied by GO_VERSIONS and VIM_VERSIONS.

# Usage:
#
#   trimLocalDockerImages.sh
#
# It is an error if GO_VERSIONS or VIM_VERSIONS is non-empty

tf=$(mktemp)
trap "rm -f $tf" EXIT

for j in $GO_VERSIONS
do
	for ii in $VALID_FLAVORS
	do
		for i in $(eval "echo \$${ii^^}_VERSIONS")
		do
			echo "${j}_${ii}_${i}_v1" >> $tf
		done
	done
done

cat $tf

toRemove=$(docker images govim/govim | tail -n +2 | grep -v base_ | grep -v -f $tf || true)

if [ "$toRemove" == "" ]
then
	echo "Nothing to trim"
	exit 0
fi

echo Will remove $(echo "$toRemove" | awk '{print $2}')
docker rmi -f $(echo "$toRemove" | awk '{print $3}')

echo You might now want to run: docker image prune
