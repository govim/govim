#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"
source "${BASH_SOURCE%/*}/tidyUp.bash"

if [ "${CI:-}" != "true" ]
then
	exit
fi

doBranchCheck

tidyUp $ARTEFACTS

# This is a way to make sure that the upload artifact
# step in CI only runs after tidyUp, since it isn't
# possible (at least currently) to check the status
# of this job step alone.
echo "CI_UPLOAD_ARTIFACTS=true" >> $GITHUB_ENV
