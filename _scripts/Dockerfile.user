FROM govim/govim:${GO_VERSION}_${VIM_FLAVOR}_${VIM_VERSION}_v1

ARG USER
ARG UID
ARG GID

ENV PATH=/vbash/bin:/home/$USER/.local/bin:$PATH
ENV GOPATH=/home/$USER/gopath

# Create group if it doesn't exist
RUN sh -c "if ! getent group $GID; then groupadd -g $GID $USER; fi" && \
    adduser --uid $UID --gid $GID --disabled-password --gecos "" $USER

# enable sudo
RUN usermod -aG sudo $USER
RUN echo "$USER ALL=(ALL:ALL) NOPASSWD: ALL" > /etc/sudoers.d/$USER

WORKDIR /home/$USER
USER $USER
