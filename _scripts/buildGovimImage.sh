#!/usr/bin/env vbash

if [ "$#" -gt 1 ]
then
	echo "We take at most one argument"
	exit 1
fi

if [ "$#" -eq 1 ]
then
	VIM_VERSION="$1"
else
	VIM_VERSION=$(echo $VIM_VERSIONS | tr ',' '\n' | tail -n 1)
fi

export VIM_VERSION

cat Dockerfile.user | envsubst '$GO_VERSION,$VIM_VERSION' | docker build -t govim --build-arg USER=$USER --build-arg UID=$UID --build-arg GID=$(id -g $USER) -
