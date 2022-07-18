FROM baiduxlab/sgx-rust:2004-1.1.3

ENV PATH="/root/.cargo/bin:$PATH"
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

ENV USER_HOME=/home/${USERNAME}/

ENV CARGO_HOME=${USER_HOME}/.cargo/bin
ENV RUSTUP_HOME=${USER_HOME}/.cargo/bin

COPY deployment/.devcontainer/library-scripts/*.sh deployment/.devcontainer/library-scripts/*.env /tmp/library-scripts/
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && /bin/bash /tmp/library-scripts/common-debian.sh "${INSTALL_ZSH}" "${USERNAME}" "${USER_UID}" "${USER_GID}" "${UPGRADE_PACKAGES}" "true" "true"
    #
    # ****************************************************************************
    # * TODO: Add any additional OS packages you want included in the definition *
    # * here. We want to do this before cleanup to keep the "layer" small.       *
    # ****************************************************************************
    # && apt-get -y install --no-install-recommends <your-package-list-here> \
RUN bash /tmp/library-scripts/rust-debian.sh "${CARGO_HOME}" "${RUSTUP_HOME}" "${USERNAME}" "true" "true"

RUN bash /tmp/library-scripts/go-debian.sh "none" "${GOROOT}" "${GOPATH}" "${USERNAME}" "false"

RUN apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/* /tmp/library-scripts

# [Optional] Uncomment this section to install additional OS packages.
# RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
#     && apt-get -y install --no-install-recommends <your-package-list-here>
RUN wget -q https://github.com/WebAssembly/wabt/releases/download/1.0.20/wabt-1.0.20-ubuntu.tar.gz && \
    tar -xf wabt-1.0.20-ubuntu.tar.gz wabt-1.0.20/bin/wat2wasm wabt-1.0.20/bin/wasm2wat && \
    mv wabt-1.0.20/bin/wat2wasm wabt-1.0.20/bin/wasm2wat /bin && \
    chmod +x /bin/wat2wasm /bin/wasm2wat && \
    rm -f wabt-1.0.20-ubuntu.tar.gz

RUN go install github.com/jteeuwen/go-bindata/go-bindata@latest && go-bindata -version

WORKDIR ${USER_HOME}

RUN git clone https://github.com/scrtlabs/SecretNetwork.git

WORKDIR ${USER_HOME}/SecretNetwork
