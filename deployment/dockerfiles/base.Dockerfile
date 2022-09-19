ARG SCRT_BASE_IMAGE_SECRETD=enigmampc/rocksdb:v6.24.2
ARG SCRT_BASE_IMAGE_ENCLAVE=baiduxlab/sgx-rust:2004-1.1.3
# enigmampc/rocksdb:v6.24.2

FROM $SCRT_BASE_IMAGE_ENCLAVE AS compile-enclave

ENV PATH="/root/.cargo/bin:$PATH"

# Set working directory for the build
WORKDIR /go/src/github.com/enigmampc/SecretNetwork/

ARG BUILD_VERSION="v0.0.0"
ARG SGX_MODE=SW
ARG FEATURES
ARG FEATURES_U

ARG API_KEY="00000000000000000000000000000000"
ARG SPID="FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"

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

RUN . /opt/sgxsdk/environment && env \
    && MITIGATION_CVE_2020_0551=LOAD VERSION=${VERSION} FEATURES=${FEATURES} FEATURES_U=${FEATURES_U} SGX_MODE=${SGX_MODE} make build-rust

ENTRYPOINT ["/bin/bash"]

FROM $SCRT_BASE_IMAGE_SECRETD AS compile-secretd

ENV GOROOT=/usr/local/go
ENV GOPATH=/go/
ENV PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

ADD https://go.dev/dl/go1.19.linux-amd64.tar.gz go.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go.linux-amd64.tar.gz
RUN go install github.com/jteeuwen/go-bindata/go-bindata@latest && go-bindata -version

# Set working directory for the build
WORKDIR /go/src/github.com/enigmampc/SecretNetwork

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

RUN ln -s /usr/lib/x86_64-linux-gnu/liblz4.so /usr/local/lib/liblz4.so  && ln -s /usr/lib/x86_64-linux-gnu/libzstd.so /usr/local/lib/libzstd.so

RUN mkdir -p /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/target/release/

COPY --from=compile-enclave /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/target/release/libgo_cosmwasm.so /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/target/release/libgo_cosmwasm.so
COPY --from=compile-enclave /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_enclave.signed.so /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_enclave.signed.so
COPY --from=compile-enclave /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_query_enclave.signed.so /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_query_enclave.signed.so

RUN cat ${SPID} > /go/src/github.com/enigmampc/SecretNetwork/ias_keys/develop/spid.txt
RUN cat ${SPID} > /go/src/github.com/enigmampc/SecretNetwork/ias_keys/develop/spid.txt
RUN cat ${SPID} > /go/src/github.com/enigmampc/SecretNetwork/ias_keys/develop/spid.txt

RUN cat ${API_KEY} > /go/src/github.com/enigmampc/SecretNetwork/ias_keys/develop/api_key.txt
RUN cat ${API_KEY} > /go/src/github.com/enigmampc/SecretNetwork/ias_keys/develop/api_key.txt
RUN cat ${API_KEY} > /go/src/github.com/enigmampc/SecretNetwork/ias_keys/develop/api_key.txt

RUN . /opt/sgxsdk/environment && env && CGO_LDFLAGS=${CGO_LDFLAGS} DB_BACKEND=${DB_BACKEND} MITIGATION_CVE_2020_0551=LOAD VERSION=${VERSION} FEATURES=${FEATURES} SGX_MODE=${SGX_MODE} make build_local_no_rust
RUN . /opt/sgxsdk/environment && env && MITIGATION_CVE_2020_0551=LOAD VERSION=${VERSION} FEATURES=${FEATURES} SGX_MODE=${SGX_MODE} make build_cli

ENTRYPOINT ["/bin/bash"]


