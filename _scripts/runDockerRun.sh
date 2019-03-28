#!/usr/bin/env bash

docker run -v $GOPATH/pkg/mod/cache/download:/cache -e GOPROXY=file:///cache -v $PWD:/home/$USER/govim -w /home/$USER/govim --rm govim ./_scripts/dockerRun.sh
