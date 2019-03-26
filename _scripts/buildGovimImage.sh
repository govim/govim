#!/usr/bin/env vbash

cat Dockerfile.user | envsubst '$GO_VERSION,$VIM_VERSION' | docker build -t govim --build-arg USER=$USER --build-arg UID=$UID --build-arg GID=$(id -g $USER) -
