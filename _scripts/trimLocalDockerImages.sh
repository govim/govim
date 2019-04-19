#!/usr/bin/env bash

set -eu

# trimLocalDockerImages trims down your local Docker image cache to contain
# just the matrix implied by GO_VERSIONS and VIM_VERSIONS.

# Usage:
#
#   trimLocalDockerImages.sh
#
# It is an error if GO_VERSIONS or VIM_VERSIONS is non-empty

if [ "${VIM_VERSIONS:-}" == "" ]
then
	echo "VIM_VERSIONS is not set"
	exit 1
fi
if [ "${GO_VERSIONS:-}" == "" ]
then
	echo "GO_VERSIONS is not set"
	exit 1
fi

tf=$(mktemp)
trap "rm -f $tf" EXIT

for i in $(echo "$VIM_VERSIONS" | tr ',' '\n')
do
	for j in $(echo "$GO_VERSIONS" | tr ',' '\n')
	do
		echo "${j}_${i}_v1" >> $tf
	done
done

toRemove=$(docker images myitcv/govim | tail -n +2 | grep -v -f $tf || true)

if [ "$toRemove" == "" ]
then
	echo "Nothing to trim"
	exit 0
fi

echo Will remove $(echo "$toRemove" | awk '{print $2}')
docker rmi $(echo "$toRemove" | awk '{print $3}')

echo You might now want to run: docker image prune
