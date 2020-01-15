#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"
source "${BASH_SOURCE%/*}/tidyUp.bash"

if [[ $# -eq 0 ]] ; then
    echo 'usage: captureLogs.sh dir command [args...]

captureLogs.sh simplifies the capture of logs from a run of testscript tests.

For example, given:

    captureLogs.sh /tmp/blah go test -count=1 ./...

gopls, govim and Vim logs will then be found beneath /tmp/blah according to
the directory structure of testscript scripts in the packages matched by ./...'
    exit -2
fi

dir="$1"
shift

if [ -d "$dir" ]
then
	now=$(date +%Y%m%d%H%M%S_%N)
	mv "$dir" "${dir}_${now}"
	echo "Moved existing $dir to ${dir}_${now}"
fi

mkdir -p "$dir"

# Run whatever commands were supplied, deliberately allowing failure
GOVIM_TESTSCRIPT_WORKDIR_ROOT="$dir" "$@" || true

tidyUp "$dir"

