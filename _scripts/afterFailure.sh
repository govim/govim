#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"

if [ "${CI:-}" != "true" ]
then
	exit
fi

# The ARTEFACTS variable set by .travis.yml cannot expand
# variables so we do that here
ARTEFACTS=$(echo $ARTEFACTS)

cd $ARTEFACTS

# Remove all the big directories first
sudo find . -type d \( -name .vim -o -name gopath \) -prune -exec rm -rf '{}' \;

# Now prune the files we don't want
sudo find . -type f -not -path "*/_tmp/govim_log" -and -not -path "*/_tmp/gopls_log" -and -not -path "*/_tmp/vim_channel_log" -exec rm '{}' \;

url=$(echo "{ \"public\": false, \"files\": { \"logs.base64\": { \"content\": \"$(find . -type f -print0 | tar -zc --null -T - | base64 | sed ':a;N;$!ba;s/\n/\\n/g')\" } } }" | curl -s -H "Content-Type: application/json" -u $GH_USER:$GH_TOKEN --request POST --data-binary "@-" https://api.github.com/gists | jq -r '.files."logs.base64".raw_url')
echo 'cd $(mktemp -d) && curl -s '$url' | base64 -d | tar -zx'
