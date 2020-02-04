#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"
source "${BASH_SOURCE%/*}/tidyUp.bash"

if [ "${CI:-}" != "true" ]
then
	exit
fi

doBranchCheck

tidyUp $ARTEFACTS
