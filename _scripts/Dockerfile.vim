ARG GH_USER
ARG GH_TOKEN
ARG GO_VERSION
ARG VBASHVERSION
FROM govim/govim:base_${GO_VERSION}_${VBASHVERSION}

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
