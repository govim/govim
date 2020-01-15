#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"

tidyUp() {
	# Fix up everything to be user-writable
	chmod -R u+w "$1"

	# Remove all the big directories first
	find "$1" -type d \( -name .vim -o -name gopath \) -prune -exec rm -rf '{}' \;

	# Now prune the files we don't want
	find "$1" -type f -not -path "*/_tmp/govim.log" -and -not -path "*/_tmp/gopls.log" -and -not -path "*/_tmp/vim_channel.log" -exec rm '{}' \;
}

