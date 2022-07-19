FROM rust-base-image AS build-env-rust

FROM enigmampc/rocksdb:v6.24.2

ENV GOROOT=/usr/local/go
ENV GOPATH=/go/
ENV PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

ADD https://go.dev/dl/go1.17.7.linux-amd64.tar.gz go1.17.7.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.17.7.linux-amd64.tar.gz
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

COPY --from=build-env-rust /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/target/release/libgo_cosmwasm.so /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/target/release/libgo_cosmwasm.so
COPY --from=build-env-rust /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_enclave.signed.so /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_enclave.signed.so
COPY --from=build-env-rust /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_query_enclave.signed.so /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_query_enclave.signed.so

COPY api_key.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/develop/
COPY spid.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/develop/
COPY api_key.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/production/
COPY spid.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/production/
COPY api_key.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/sw_dummy/
COPY spid.txt /go/src/github.com/enigmampc/SecretNetwork/ias_keys/sw_dummy/

RUN . /opt/sgxsdk/environment && env && CGO_LDFLAGS=${CGO_LDFLAGS} DB_BACKEND=${DB_BACKEND} MITIGATION_CVE_2020_0551=LOAD VERSION=${VERSION} FEATURES=${FEATURES} SGX_MODE=${SGX_MODE} make build_local_no_rust
RUN . /opt/sgxsdk/environment && env && MITIGATION_CVE_2020_0551=LOAD VERSION=${VERSION} FEATURES=${FEATURES} SGX_MODE=${SGX_MODE} make build_cli

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