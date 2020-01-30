#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"
source "${BASH_SOURCE%/*}/tidyUp.bash"

if [ "${CI:-}" != "true" ]
then
	exit
fi

cd $ARTEFACTS

tidyUp .

url=$(echo "{ \"public\": false, \"files\": { \"logs.base64\": { \"content\": \"$(find . -type f -print0 | tar -zc --null -T - | base64 | sed ':a;N;$!ba;s/\n/\\n/g')\" } } }" | curl -s -H "Content-Type: application/json" -u $GH_USER:$GH_TOKEN --request POST --data-binary "@-" https://api.github.com/gists | jq -r '.files."logs.base64".raw_url')
echo 'cd $(mktemp -d) && curl -s '$url' | base64 -d | tar -zx'
