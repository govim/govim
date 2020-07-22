#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"

# Update the :latest tag for govim images. By default only runs on CI whilst
# building against the main branch. This isn't totally foolproof, but good
# enough for what we need
#
# Usage:
#
#   _scripts/tagDockerLatest.sh [-f]
#
# The -f flag forces the update of latest, even when we aren't running on CI
# against the main branch

# Only run for the build matrix entry that corresponds to
# the tag we are going to create
if [[ "${CI:-}" == "true" ]] && ([[ $VIM_FLAVOR != "vim" ]] || [[ $VIM_VERSION != $MAX_VIM_VERSION ]] || [[ $GO_VERSION != $MAX_GO_VERSION ]])
then
	echo "Skipping tagging of :latest docker image; this build matrix entry does not correspond to the tag we are creating"
	exit 0
fi

# If we are not on CI, then only run if -f is supplied
if [[ "${CI:-}" != "true" ]] && [[ "${1:-}" != "-f" ]]
then
	echo "Cowardly refusing to tag :latest; not on CI building main branch, and no -f supplied"
	exit 1
fi

# If we are on CI, only tag if we are on the main branch
if [[ "${CI:-}" == "true" ]] && [[ "${GITHUB_REF:-}" != "refs/heads/main" ]]
then
	echo "Skipping tagging of :latest docker image; we are not building main branch"
	exit 0
fi

if [[ "${CI:-}" == "true" ]]
then
	docker login -u $DOCKER_HUB_USER -p $DOCKER_HUB_TOKEN
fi

docker pull govim/govim:${MAX_GO_VERSION}_vim_${MAX_VIM_VERSION}_v1 ##
docker tag govim/govim:${MAX_GO_VERSION}_vim_${MAX_VIM_VERSION}_v1 govim/govim:latest-vim ##
docker push govim/govim:latest-vim
