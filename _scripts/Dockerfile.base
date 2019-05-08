FROM buildpack-deps:18.04

RUN apt-get update && \
  apt-get -y install sudo apt-utils git jq curl libncurses5-dev gcc rsync libgtk2.0-dev xvfb && \
  apt-get clean

RUN git config --global advice.detachedHead false

ARG GO_VERSION
RUN curl -sL https://dl.google.com/go/${GO_VERSION}.linux-amd64.tar.gz | tar -C / -zx
ENV PATH=/go/bin:$PATH

ARG VBASHVERSION
RUN cd $(mktemp -d) && \
  GO111MODULE=on go mod init mod && \
  GOPROXY=https://proxy.golang.org go get github.com/myitcv/vbash@$VBASHVERSION && \
  GOBIN=/usr/bin go install github.com/myitcv/vbash
