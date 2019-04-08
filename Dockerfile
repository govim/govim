FROM buildpack-deps:18.04

RUN apt-get update
RUN apt-get -y install sudo apt-utils git jq curl libncurses5-dev gcc rsync libgtk2.0-dev xvfb

RUN git config --global advice.detachedHead false

ARG GO_VERSION
RUN curl -sL https://dl.google.com/go/${GO_VERSION}.linux-amd64.tar.gz | tar -C / -zx
ENV PATH=/go/bin:$PATH

ARG VBASHVERSION
RUN cd $(mktemp -d) && \
  GO111MODULE=on go mod init mod && \
  go get github.com/myitcv/vbash@$VBASHVERSION && \
  GOBIN=/usr/bin go install github.com/myitcv/vbash

ARG VIM_VERSION
RUN cd /tmp && \
  git clone https://github.com/vim/vim && \
  cd vim && \
  git checkout $VIM_VERSION && \
  ./configure --prefix=/vim --enable-gui=gtk2 --disable-darwin --disable-selinux --disable-netbeans && \
  make -j$(nproc --all) install

ENV PATH=/vim/bin:$PATH

