# Simple usage with a mounted data directory:
# > docker build -t enigma .
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.secretd:/root/.secretd -v ~/.secretcli:/root/.secretcli enigma secretd init
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.secretd:/root/.secretd -v ~/.secretcli:/root/.secretcli enigma secretd start
FROM baiduxlab/sgx-rust:2004-1.1.3 AS build-env-rust-go

ENV PATH="/root/.cargo/bin:$PATH"
ENV GOROOT=/usr/local/go
ENV GOPATH=/go/
ENV PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

ADD https://go.dev/dl/go1.17.7.linux-amd64.tar.gz go1.17.7.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.17.7.linux-amd64.tar.gz
RUN go get -u github.com/jteeuwen/go-bindata/...

RUN wget -q https://github.com/WebAssembly/wabt/releases/download/1.0.20/wabt-1.0.20-ubuntu.tar.gz && \
    tar -xf wabt-1.0.20-ubuntu.tar.gz wabt-1.0.20/bin/wat2wasm wabt-1.0.20/bin/wasm2wat && \
    mv wabt-1.0.20/bin/wat2wasm wabt-1.0.20/bin/wasm2wat /bin && \
    chmod +x /bin/wat2wasm /bin/wasm2wat && \
    rm -f wabt-1.0.20-ubuntu.tar.gz

### Install rocksdb

RUN apt-get update &&  \
    apt-get install -y \
    # apt-get install -y --no-install-recommends \
#    libgflags-dev \
#    libsnappy-dev \
    zlib1g-dev
#    libbz2-dev \
#    liblz4-dev \
#    libzstd-dev

RUN git clone https://github.com/facebook/rocksdb.git

WORKDIR rocksdb

RUN git checkout v6.24.2
RUN export CXXFLAGS='-Wno-error=deprecated-copy -Wno-error=pessimizing-move -Wno-error=class-memaccess -lstdc++ -lm -lz -lbz2 -lzstd -llz4'
RUN DEBUG_LEVEL=0 make static_lib -j 24
RUN make install-static INSTALL_PATH=/usr
# rm -rf /tmp/rocksdb
# Set working directory for the build
WORKDIR /go/src/github.com/enigmampc/SecretNetwork/

ARG BUILD_VERSION="v0.0.0"
ARG SGX_MODE=SW
ARG FEATURES
ARG FEATURES_U
ARG WITH_ROCKSDB

ENV WITH_ROCKSDB=${WITH_ROCKSDB}
ENV VERSION=${BUILD_VERSION}
ENV SGX_MODE=${SGX_MODE}
ENV FEATURES=${FEATURES}
ENV FEATURES_U=${FEATURES_U}
ENV MITIGATION_CVE_2020_0551=LOAD



COPY third_party/build third_party/build

# Add source files
COPY go-cosmwasm go-cosmwasm/
COPY cosmwasm cosmwasm/

WORKDIR /go/src/github.com/enigmampc/SecretNetwork/

COPY deployment/docker/MakefileCopy Makefile

# RUN make clean
RUN make vendor

WORKDIR /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm

COPY api_key.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/develop/
COPY spid.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/develop/
COPY api_key.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/production/
COPY spid.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/production/
COPY api_key.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/sw_dummy/
COPY spid.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/sw_dummy/

RUN . /opt/sgxsdk/environment && env \
    && MITIGATION_CVE_2020_0551=LOAD VERSION=${VERSION} FEATURES=${FEATURES} FEATURES_U=${FEATURES_U} SGX_MODE=${SGX_MODE} make build-rust

# Set working directory for the build
WORKDIR /go/src/github.com/enigmampc/SecretNetwork

#RUN rustup target add wasm32-unknown-unknown
#
#COPY install-wasm-tools.sh .
#RUN ./install-wasm-tools.sh

# RUN make build-test-contract

# Add source files
COPY go-cosmwasm go-cosmwasm
# This is due to some esoteric docker bug with the underlying filesystem, so until I figure out a better way, this should be a workaround
RUN true
COPY x x
RUN true
COPY types types
RUN true
COPY app app
COPY go.mod .
COPY go.sum .
COPY cmd cmd
COPY Makefile .
RUN true
COPY client client
# COPY /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/libgo_cosmwasm.so go-cosmwasm/api
RUN export CGO_LDFLAGS="-L/usr/local/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lzstd -llz4"
RUN . /opt/sgxsdk/environment && env && CGO_LDFLAGS="-L/usr/local/lib -lrocksdb -lstdc++ -lm -lz" MITIGATION_CVE_2020_0551=LOAD VERSION=${VERSION} FEATURES=${FEATURES} SGX_MODE=${SGX_MODE} make build_local_no_rust
RUN . /opt/sgxsdk/environment && env && CGO_LDFLAGS="-L/usr/local/lib -lrocksdb -lstdc++ -lm -lz" MITIGATION_CVE_2020_0551=LOAD VERSION=${VERSION} FEATURES=${FEATURES} SGX_MODE=${SGX_MODE} make build_cli

RUN rustup target add wasm32-unknown-unknown && make build-test-contract

# workaround because paths seem kind of messed up
# RUN cp /opt/sgxsdk/lib64/libsgx_urts_sim.so /usr/lib/libsgx_urts_sim.so
# RUN cp /opt/sgxsdk/lib64/libsgx_uae_service_sim.so /usr/lib/libsgx_uae_service_sim.so
# RUN cp /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/target/release/libgo_cosmwasm.so /usr/lib/libgo_cosmwasm.so
# RUN cp /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_enclave.signed.so /usr/lib/librust_cosmwasm_enclave.signed.so
# RUN cp /go/src/github.com/enigmampc/SecretNetwork/cosmwasm/packages/wasmi-runtime/librust_cosmwasm_enclave.signed.so x/compute/internal/keeper
# RUN mkdir -p /go/src/github.com/enigmampc/SecretNetwork/x/compute/internal/keeper/.sgx_secrets

#COPY deployment/ci/go-tests.sh .
#
#RUN chmod +x go-tests.sh

# ENTRYPOINT ["/bin/bash", "go-tests.sh"]
ENTRYPOINT ["/bin/bash"]