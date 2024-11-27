#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"

version="--branch $1"
if [[ "${2:-}" == "schedule" ]]; then
	version=""
fi

pushd $(mktemp -d) > /dev/null
git clone $version --depth 1 https://github.com/vim/vim
cd vim
./configure --prefix=$HOME/vim --enable-gui=gtk2 --disable-darwin --disable-selinux --disable-netbeans
make -j
make install

export PATH="$HOME/vim/bin:$PATH"

vim --version

popd > /dev/null

go test ./...
