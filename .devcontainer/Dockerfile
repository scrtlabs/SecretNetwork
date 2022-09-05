FROM ubuntu:focal

ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH \
    RUST_VERSION=1.62.0

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    #### Base utilities ####
    wget \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

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

# ENV PATH="/root/.cargo/bin:$PATH"

ENV GOROOT=/usr/local/go
ENV GOPATH=/go/
ENV PATH=$PATH:/usr/local/go/bin:$GOPATH/bin
ENV GO111MODULE=auto

# Use the [Option] comment to specify true/false arguments that should appear in VS Code UX
#
# [Option] Install zsh
ARG INSTALL_ZSH="true"
# [Option] Upgrade OS packages to their latest versions
ARG UPGRADE_PACKAGES="false"

# Install needed packages and setup non-root user. Use a separate RUN statement to add your own dependencies.
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

ENV USER_HOME=/home/${USERNAME}

# ENV CARGO_HOME=${USER_HOME}/.cargo/bin
# ENV PATH="${CARGO_HOME}:$PATH"

# ENV RUSTUP_HOME=${USER_HOME}/.cargo/bin


COPY library-scripts/*.sh library-scripts/*.env /tmp/library-scripts/
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && /bin/bash /tmp/library-scripts/common-debian.sh "${INSTALL_ZSH}" "${USERNAME}" "${USER_UID}" "${USER_GID}" "${UPGRADE_PACKAGES}" "true" "true"
    #
    # ****************************************************************************
    # * TODO: Add any additional OS packages you want included in the definition *
    # * here. We want to do this before cleanup to keep the "layer" small.       *
    # ****************************************************************************
    # && apt-get -y install --no-install-recommends <your-package-list-here> \
# USER $USERNAME

# Install rust as non-root user
# RUN curl https://sh.rustup.rs -sSf | bash -s -- -y

# USER root

RUN bash /tmp/library-scripts/rust-debian.sh "${CARGO_HOME}" "${RUSTUP_HOME}" "${USERNAME}" "true" "true"

# Install Go
#ADD https://go.dev/dl/go1.18.4.linux-amd64.tar.gz go1.18.4.linux-amd64.tar.gz
#RUN tar -C /usr/local -xzf go1.18.4.linux-amd64.tar.gz
#
#RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" "$GOPATH/pkg" && chmod -R 777 "$GOPATH"
#RUN chown ${USERNAME}:${USERNAME} /go -R

RUN bash /tmp/library-scripts/go-debian.sh "1.18.4" "${GOROOT}" "${GOPATH}" "${USERNAME}" "true" "true" && \
    go install github.com/jteeuwen/go-bindata/go-bindata@latest && go-bindata -version

RUN apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/* /tmp/library-scripts

# [Optional] Uncomment this section to install additional OS packages.
# RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
#     && apt-get -y install --no-install-recommends <your-package-list-here>
RUN wget -q https://github.com/WebAssembly/wabt/releases/download/1.0.20/wabt-1.0.20-ubuntu.tar.gz && \
    tar -xf wabt-1.0.20-ubuntu.tar.gz wabt-1.0.20/bin/wat2wasm wabt-1.0.20/bin/wasm2wat && \
    mv wabt-1.0.20/bin/wat2wasm wabt-1.0.20/bin/wasm2wat /bin && \
    chmod +x /bin/wat2wasm /bin/wasm2wat && \
    rm -f wabt-1.0.20-ubuntu.tar.gz

USER root

# SGX version parameters
ARG SGX_VERSION=2.12.100.3
ARG OS_REVESION=focal1

# RUN echo "deb http://deb.debian.org/debian bullseye-backports main" | sudo tee /etc/apt/sources.list.d/bullseye-backports.list

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    #### Base utilities ####
    gdebi \
    libprotobuf17 \
    autoconf \
    libtool \
    automake \
    clang \
    #### SGX installer dependencies ####
    g++ make libcurl4 libssl1.1 && \
    rm -rf /var/lib/apt/lists/*


#RUN wget -O /tmp/libprotobuf10_3.0.0-9_amd64.deb http://ftp.br.debian.org/debian/pool/main/p/protobuf/libprotobuf10_3.0.0-9_amd64.deb
#RUN yes | gdebi /tmp/libprotobuf10_3.0.0-9_amd64.deb

WORKDIR /root

# Must create /etc/init or enclave-common install will fail
RUN mkdir /etc/init && \
    mkdir sgx

# SGX Binaries
ARG SGX_VERSION=2.12.100.3
ARG OS_REVESION=focal1

##### Install SGX Binaries ######
ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/debian_pkgs/libs/libsgx-enclave-common/libsgx-enclave-common_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/debian_pkgs/libs/libsgx-urts/libsgx-urts_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/debian_pkgs/libs/libsgx-uae-service/libsgx-uae-service_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/debian_pkgs/libs/libsgx-quote-ex/libsgx-quote-ex_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/debian_pkgs/libs/libsgx-epid/libsgx-epid_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/debian_pkgs/libs/libsgx-launch/libsgx-launch_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/


RUN dpkg -i ./sgx/libsgx-enclave-common_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-urts_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-launch_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-epid_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-quote-ex_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-uae-service_${SGX_VERSION}-${OS_REVESION}_amd64.deb

ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/sgx_linux_x64_sdk_${SGX_VERSION}.bin ./sgx/
# ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/sgx_linux_x64_driver_2.11.0_4505f07.bin ./sgx/

RUN chmod +x ./sgx/sgx_linux_x64_sdk_${SGX_VERSION}.bin
RUN echo -e 'no\n/opt' | ./sgx/sgx_linux_x64_sdk_${SGX_VERSION}.bin && \
    echo 'source /opt/sgxsdk/environment' >> /etc/bash.bashrc && \
    echo 'export SGX_MODE=SW' >> /etc/bash.bashrc \
    rm -rf ./sgx/*

# ENV LD_LIBRARY_PATH=/opt/sgxsdk/libsgx-enclave-common/

RUN echo 'export SCRT_ENCLAVE_DIR=/home/vscode/SecretNetwork/go-cosmwasm/' >> /etc/bash.bashrc

USER $USERNAME

WORKDIR ${USER_HOME}

RUN cargo install xargo --version 0.3.25
RUN git clone https://github.com/scrtlabs/SecretNetwork.git

WORKDIR ${USER_HOME}/SecretNetwork

# USER root
# get rust
RUN rustup target add wasm32-unknown-unknown && rustup component add rust-src rust-analysis clippy
USER root
# fuck you permissions
RUN chown $USERNAME:$USERNAME /go -R

# '-l': see https://docs.docker.com/develop/develop-images/dockerfile_best-practices/#user
RUN useradd -l -u 33333 -G sudo -md /home/gitpod -s /bin/bash -p gitpod gitpod \
    # passwordless sudo for users in the 'sudo' group
    && sed -i.bkp -e 's/%sudo\s\+ALL=(ALL\(:ALL\)\?)\s\+ALL/%sudo ALL=NOPASSWD:ALL/g' /etc/sudoers \
    # To emulate the workspace-session behavior within dazzle build env
    && mkdir /workspace && chown -hR gitpod:gitpod /workspace
