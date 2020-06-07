#!/usr/bin/env bash

source "${BASH_SOURCE%/*}/common.bash"

pushd $(mktemp -d) > /dev/null
git clone https://github.com/vim/vim
cd vim
./configure --prefix=$HOME/vim --enable-gui=gtk2 --disable-darwin --disable-selinux --disable-netbeans
make -j
make install

export PATH="$HOME/vim/bin:$PATH"

vim --version

popd > /dev/null

go test ./...
