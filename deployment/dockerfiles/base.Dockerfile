# Simple usage with a mounted data directory:
# > docker build -t enigma .
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.secretd:/root/.secretd -v ~/.secretcli:/root/.secretcli enigma secretd init
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.secretd:/root/.secretd -v ~/.secretcli:/root/.secretcli enigma secretd start
FROM enigmampc/rocksdb:v6.24.2-1.1.5 AS build-env-rust-go

ENV PATH="/root/.cargo/bin:$PATH"
ENV GOROOT=/usr/local/go
ENV GOPATH=/go/
ENV PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

ADD https://go.dev/dl/go1.19.linux-amd64.tar.gz go.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go.linux-amd64.tar.gz
RUN go install github.com/jteeuwen/go-bindata/go-bindata@latest && go-bindata -version

RUN wget -q https://github.com/WebAssembly/wabt/releases/download/1.0.20/wabt-1.0.20-ubuntu.tar.gz && \
    tar -xf wabt-1.0.20-ubuntu.tar.gz wabt-1.0.20/bin/wat2wasm wabt-1.0.20/bin/wasm2wat && \
    mv wabt-1.0.20/bin/wat2wasm wabt-1.0.20/bin/wasm2wat /bin && \
    chmod +x /bin/wat2wasm /bin/wasm2wat && \
    rm -f wabt-1.0.20-ubuntu.tar.gz


#### Install rocksdb deps

RUN apt-get update &&  \
    apt-get install -y --no-install-recommends \
    zlib1g-dev \
    libbz2-dev \
    liblz4-dev \
    libzstd-dev
#


# rm -rf /tmp/rocksdb
# Set working directory for the build
WORKDIR /go/src/github.com/enigmampc/SecretNetwork/

ARG BUILD_VERSION="v0.0.0"
ARG SGX_MODE=SW
ARG FEATURES
ARG FEATURES_U
ARG DB_BACKEND=goleveldb
ARG CGO_LDFLAGS

ENV VERSION=${BUILD_VERSION}
ENV SGX_MODE=${SGX_MODE}
ENV FEATURES=${FEATURES}
ENV FEATURES_U=${FEATURES_U}
ENV MITIGATION_CVE_2020_0551=LOAD

COPY rust-toolchain rust-toolchain
RUN rustup component add rust-src
RUN cargo install xargo --version 0.3.25

COPY third_party third_party

# Add source files
COPY go-cosmwasm go-cosmwasm/
COPY cosmwasm cosmwasm/

# this is here so we don't have to recompile every time the makefile changes
COPY deployment/docker/MakefileCopy Makefile

WORKDIR /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm

RUN . /opt/sgxsdk/environment && env \
    && MITIGATION_CVE_2020_0551=LOAD VERSION=${VERSION} FEATURES=${FEATURES} FEATURES_U=${FEATURES_U} SGX_MODE=${SGX_MODE} make build-rust

# switch back to root directory and copy all the sdk stuff
WORKDIR /go/src/github.com/enigmampc/SecretNetwork/

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

RUN ln -s /usr/lib/x86_64-linux-gnu/liblz4.so /usr/local/lib/liblz4.so  && ln -s /usr/lib/x86_64-linux-gnu/libzstd.so /usr/local/lib/libzstd.so

COPY ias_keys ias_keys

COPY api_key.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/production/
COPY spid.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/production/

RUN . /opt/sgxsdk/environment && CGO_LDFLAGS=${CGO_LDFLAGS} DB_BACKEND=${DB_BACKEND} VERSION=${VERSION} FEATURES=${FEATURES} SGX_MODE=${SGX_MODE} make build_local_no_rust
RUN . /opt/sgxsdk/environment && VERSION=${VERSION} FEATURES=${FEATURES} SGX_MODE=${SGX_MODE} make build_cli

# RUN rustup target add wasm32-unknown-unknown && apt update -y && apt install clang -y && make build-test-contract

# ENTRYPOINT ["/bin/bash", "go-tests.sh"]
ENTRYPOINT ["/bin/bash"]
