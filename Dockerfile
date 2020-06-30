# Simple usage with a mounted data directory:
# > docker build -t enigma .
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.secretd:/root/.secretd -v ~/.secretcli:/root/.secretcli enigma secretd init
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.secretd:/root/.secretd -v ~/.secretcli:/root/.secretcli enigma secretd start
FROM baiduxlab/sgx-rust:1804-1.1.2 AS build-env-rust-go

ENV PATH="/root/.cargo/bin:$PATH"
ENV GOROOT=/usr/local/go
ENV GOPATH=/go/
ENV PATH=$PATH:/usr/local/go/bin:$GOPATH/bin


RUN curl -O https://dl.google.com/go/go1.14.2.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.14.2.linux-amd64.tar.gz
# Set working directory for the build

WORKDIR /go/src/github.com/enigmampc/SecretNetwork/

ARG SGX_MODE=SW
ENV SGX_MODE=${SGX_MODE}
ENV MITIGATION_CVE_2020_0551=LOAD

COPY third_party/build third_party/build

# Add source files
COPY go-cosmwasm/ go-cosmwasm/
COPY cosmwasm/ cosmwasm/

WORKDIR /go/src/github.com/enigmampc/SecretNetwork/

COPY Makefile Makefile

# RUN make clean
RUN make vendor

WORKDIR /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm
RUN . /opt/sgxsdk/environment && env && SGX_MODE=${SGX_MODE} make build-rust

# Set working directory for the build
WORKDIR /go/src/github.com/enigmampc/SecretNetwork

# Add source files
COPY go-cosmwasm go-cosmwasm
# This is due to some esoteric docker bug with the underlying filesystem, so until I figure out a better way, this should be a workaround
RUN true
COPY x x
RUN true
COPY types types
RUN true
COPY app.go .
COPY go.mod .
COPY go.sum .
COPY cmd cmd
COPY Makefile .

# COPY /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/libgo_cosmwasm.so go-cosmwasm/api

RUN . /opt/sgxsdk/environment && env && MITIGATION_CVE_2020_0551=LOAD SGX_MODE=${SGX_MODE} make build_local_no_rust

# Final image
FROM cashmaney/enigma-sgx-base

# wasmi-sgx-test script requirements
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    #### Base utilities ####
    jq \
    wget \
    curl && \
    rm -rf /var/lib/apt/lists/*


ARG SGX_MODE=SW
ENV SGX_MODE=${SGX_MODE}

ARG SECRET_NODE_TYPE=BOOTSTRAP
ENV SECRET_NODE_TYPE=${SECRET_NODE_TYPE}

ENV SCRT_ENCLAVE_DIR=/usr/lib/

# workaround because paths seem kind of messed up
RUN cp /opt/sgxsdk/lib64/libsgx_urts_sim.so /usr/lib/libsgx_urts_sim.so
RUN cp /opt/sgxsdk/lib64/libsgx_uae_service_sim.so /usr/lib/libsgx_uae_service_sim.so

# Install ca-certificates
WORKDIR /root

# Copy over binaries from the build-env
COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/target/release/libgo_cosmwasm.so /usr/lib/
COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_enclave.signed.so /usr/lib/
COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/secretd /usr/bin/secretd
COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/secretcli /usr/bin/secretcli

COPY ./x/compute/internal/keeper/testdata/erc20.wasm erc20.wasm

# COPY ./packaging_docker/devnet_init.sh .
COPY packaging_docker/ci/wasmi-sgx-test.sh .
COPY packaging_docker/ci/bootstrap_init.sh .
COPY packaging_docker/ci/node_init.sh .
COPY packaging_docker/ci/startup.sh .
COPY packaging_docker/ci/node_key.json .

RUN chmod +x /usr/bin/secretd
RUN chmod +x /usr/bin/secretcli
RUN chmod +x wasmi-sgx-test.sh
RUN chmod +x bootstrap_init.sh
RUN chmod +x startup.sh
RUN chmod +x node_init.sh


RUN mkdir -p /root/.secretd/.compute/
RUN mkdir -p /root/.sgx_secrets/
RUN mkdir -p /root/.secretd/.node/
# COPY ./packaging_docker/seed.json /root/.secretd/.compute/seed.json

COPY api_key.txt /root/
COPY spid.txt /root/

#ENV LD_LIBRARY_PATH=/opt/sgxsdk/libsgx-enclave-common/:/opt/sgxsdk/lib64/

# Run secretd by default, omit entrypoint to ease using container with secretcli
ENTRYPOINT ["/bin/bash", "startup.sh"]
