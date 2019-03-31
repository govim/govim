FROM buildpack-deps:18.04

RUN apt-get update
RUN apt-get -y install sudo apt-utils git jq curl libncurses5-dev gcc rsync

RUN git config --global advice.detachedHead false

ARG GOBINPATH
COPY $GOBINPATH /usr/bin/

ARG VBASHPATH
COPY $VBASHPATH /usr/bin/

ARG GO_VERSION
RUN curl -sL https://dl.google.com/go/${GO_VERSION}.linux-amd64.tar.gz | tar -C / -zx
ENV PATH=/go/bin:$PATH

ARG VIM_VERSION
RUN cd /tmp && \
  git clone https://github.com/vim/vim && \
  cd vim && \
  git checkout $VIM_VERSION && \
  ./configure --prefix=/vim --disable-darwin --disable-selinux --disable-netbeans --enable-gui=no && \
  make -j$(nproc --all) install

ENV PATH=/vim/bin:$PATH

