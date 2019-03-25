#!/usr/bin/env vbash

set -eu

vbashPath=$(realpath --relative-to=$PWD $(gobin -m -p github.com/myitcv/vbash))
gobinPath=$(realpath --relative-to=$PWD $(gobin -m -p github.com/myitcv/gobin))

docker build --build-arg "GOBINPATH=$gobinPath" --build-arg "VBASHPATH=$vbashPath" --build-arg "GO_VERSION=$GO_VERSION" --build-arg "VIM_VERSION=$VIM_VERSION" -t myitcv/govim:${GO_VERSION}_${VIM_VERSION} .
