FROM baiduxlab/sgx-rust:2004-1.1.3

RUN mkdir -p /opt/secret/.sgx_secrets

COPY scripts/install-wasm-tools.sh install-wasm-tools.sh
RUN chmod +x install-wasm-tools.sh
RUN ./install-wasm-tools.sh

RUN $HOME/.cargo/bin/rustup install 1.61
RUN $HOME/.cargo/bin/rustup target add wasm32-unknown-unknown

ENV GOROOT=/usr/local/go
ENV GOPATH=/go/
ENV PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

ADD https://go.dev/dl/go1.19.linux-amd64.tar.gz go.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go.linux-amd64.tar.gz

ENTRYPOINT ["/bin/bash"]
