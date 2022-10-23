FROM ghcr.io/scrtlabs/localsecret:v1.4.1-patch.3

### Install Sudo ###

RUN apt-get update && \
    apt-get install -yq --no-install-recommends sudo

### Install Rust ###

ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH \
    RUST_VERSION=1.63.0
ENV dpkgArch=amd64
ENV rustArch='x86_64-unknown-linux-gnu'
ENV rustupSha256='3dc5ef50861ee18657f9db2eeb7392f9c2a6c95c90ab41e45ab4ca71476b4338'

RUN set -eux; \
    url="https://static.rust-lang.org/rustup/archive/1.24.3/${rustArch}/rustup-init"; \
    wget "$url" --no-check-certificate; \
    echo "${rustupSha256} *rustup-init" | sha256sum -c -; \
    chmod +x rustup-init; \
    ./rustup-init -y --no-modify-path --profile minimal --default-toolchain $RUST_VERSION --default-host ${rustArch}; \
    rm rustup-init; \
    chmod -R a+w $RUSTUP_HOME $CARGO_HOME; \
    rustup --version; \
    cargo --version; \
    rustc --version;


### Gitpod user ###

# '-l': see https://docs.docker.com/develop/develop-images/dockerfile_best-practices/#user
RUN useradd -l -u 33333 -G sudo -md /home/gitpod -s /bin/bash -p gitpod gitpod \
    # passwordless sudo for users in the 'sudo' group
    && sed -i.bkp -e 's/%sudo\s\+ALL=(ALL\(:ALL\)\?)\s\+ALL/%sudo ALL=NOPASSWD:ALL/g' /etc/sudoers
ENV HOME=/home/gitpod
WORKDIR $HOME

# Install needed packages and setup non-root user. Use a separate RUN statement to add your own dependencies.
ARG USERNAME=gitpod
ARG USER_UID=33333
ARG USER_GID=$USER_UID

ENV USER_HOME=/home/${USERNAME}

RUN apt-get install -yq clang binaryen

### Setup Rust and More common packages ###

COPY .devcontainer/library-scripts/*.sh .devcontainer/library-scripts/*.env /tmp/library-scripts/
RUN export DEBIAN_FRONTEND=noninteractive \
    && /bin/bash /tmp/library-scripts/common-debian.sh "${INSTALL_ZSH}" "${USERNAME}" "${USER_UID}" "${USER_GID}" "${UPGRADE_PACKAGES}" "true" "true"

RUN bash /tmp/library-scripts/rust-debian.sh "${CARGO_HOME}" "${RUSTUP_HOME}" "${USERNAME}" "true" "true"

### Setup permissions for Secretd ###

RUN chown -R gitpod:gitpod /opt/secret
RUN echo 'alias secretcli=secretd' >> /etc/bash.bashrc

### Gitpod user (2) ###
USER gitpod

RUN rustup target add wasm32-unknown-unknown && rustup component add rust-src rust-analysis clippy

RUN mkdir -p $HOME/.secretd/.compute/
RUN mkdir -p $HOME/.secretd/.node/
RUN mkdir -p $HOME/config/

COPY deployment/docker/devimage/bootstrap_init_no_stop.sh bootstrap_init.sh
COPY deployment/docker/devimage/faucet/faucet_server.js .

RUN sudo npm cache clean -f && sudo npm install -g n && sudo n 14.19

CMD ["./bootstrap_init.sh"]