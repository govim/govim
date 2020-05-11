FROM buildpack-deps:18.04

RUN apt-get update && \
  apt-get -y install sudo apt-utils git jq curl libncurses5-dev gcc rsync libgtk2.0-dev xvfb && \
  apt-get clean

RUN git config --global advice.detachedHead false

ARG GH_USER
ARG GH_TOKEN
RUN echo -e "machine github.com\n  login $GH_USER\n  password $GH_TOKEN" >> ~/.netrc
RUN echo -e "machine githubusercontent.com\n  login $GH_USER\n  password $GH_TOKEN" >> ~/.netrc

ARG VIM_VERSION
RUN cd /tmp && \
  git clone https://github.com/vim/vim && \
  cd vim && \
  git checkout $VIM_VERSION && \
  ./configure --prefix=/vim --enable-gui=gtk2 --disable-darwin --disable-selinux --disable-netbeans && \
  make -j$(nproc --all) install

ENV PATH=/vim/bin:$PATH

RUN rm ~/.netrc

ARG GO_VERSION
RUN curl -sL https://dl.google.com/go/${GO_VERSION}.linux-amd64.tar.gz | tar -C / -zx
ENV PATH=/go/bin:$PATH

ARG VBASHVERSION
RUN cd $(mktemp -d) && \
  GO111MODULE=on go mod init mod && \
  GOPROXY=https://proxy.golang.org go get github.com/myitcv/vbash@$VBASHVERSION && \
  GOBIN=/usr/bin go install github.com/myitcv/vbash

