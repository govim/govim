ARG GH_USER
ARG GH_TOKEN
ARG GO_VERSION
ARG VBASHVERSION
FROM govim/govim:base_${GO_VERSION}_${VBASHVERSION}

RUN echo -e "machine github.com\n  login $GH_USER\n  password $GH_TOKEN" >> ~/.netrc
RUN echo -e "machine githubusercontent.com\n  login $GH_USER\n  password $GH_TOKEN" >> ~/.netrc

RUN apt-get update && \
    apt-get install -y \
    autoconf \
    automake \
    cmake \
    g++ pkg-config \
    gettext \
    libtool \
    libtool-bin \
    ninja-build \
    unzip \
    && apt-get clean

ARG VIM_VERSION
RUN cd /tmp && \
  git clone https://github.com/neovim/neovim && \
  cd neovim && \
  git checkout $VIM_VERSION && \
  make CMAKE_INSTALL_PREFIX=/neovim && \
  make install

ENV PATH=/neovim/bin:$PATH

RUN rm ~/.netrc
