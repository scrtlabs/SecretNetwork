ARG SCRT_BIN_IMAGE=rust-go-base-image
ARG SCRT_BASE_IMAGE=enigmampc/enigma-sgx-base:2004-1.1.5

FROM $SCRT_BIN_IMAGE AS build-env-rust-go

# Final image
FROM $SCRT_BASE_IMAGE as build-node

# wasmi-sgx-test script requirements
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    #### Base utilities ####
    jq \
    openssl \
    curl \
    wget \
    libsnappy-dev \
    libgflags-dev \
    bash-completion && \
    rm -rf /var/lib/apt/lists/*

RUN echo "source /etc/profile.d/bash_completion.sh" >> ~/.bashrc

RUN curl -sL https://deb.nodesource.com/setup_16.x | bash - && \
    apt-get update && \
    apt-get install -y nodejs && \
    npm i -g local-cors-proxy

ARG SGX_MODE=SW
ENV SGX_MODE=${SGX_MODE}

ARG SECRET_NODE_TYPE=BOOTSTRAP
ENV SECRET_NODE_TYPE=${SECRET_NODE_TYPE}

ENV PKG_CONFIG_PATH=""
ENV SCRT_ENCLAVE_DIR=/usr/lib/

# workaround because paths seem kind of messed up
RUN ln -s /opt/sgxsdk/lib64/libsgx_urts_sim.so /usr/lib/x86_64-linux-gnu/libsgx_urts_sim.so
RUN ln -s /opt/sgxsdk/lib64/libsgx_uae_service_sim.so /usr/lib/x86_64-linux-gnu/libsgx_uae_service_sim.so

# Install ca-certificates
WORKDIR /root

# Copy over binaries from the build-env
COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/target/release/libgo_cosmwasm.so /usr/lib/
COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_enclave.signed.so /usr/lib/
#COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_query_enclave.signed.so /usr/lib/
COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/secretd /usr/bin/secretd

COPY deployment/docker/bootstrap/bootstrap_init.sh .
COPY deployment/docker/node/node_init.sh .
COPY deployment/docker/startup.sh .
COPY deployment/docker/node_key.json .

RUN chmod +x /usr/bin/secretd
RUN chmod +x bootstrap_init.sh
RUN chmod +x startup.sh
RUN chmod +x node_init.sh

RUN secretd completion > /root/secretd_completion

RUN echo 'source /root/secretd_completion' >> ~/.bashrc

RUN mkdir -p /root/.secretd/.compute/
RUN mkdir -p /opt/secret/.sgx_secrets/
RUN mkdir -p /root/.secretd/.node/
RUN mkdir -p /root/config/


####### Node parameters
ARG MONIKER=default
ARG CHAINID=secretdev-1
ARG GENESISPATH=https://raw.githubusercontent.com/enigmampc/SecretNetwork/master/secret-testnet-genesis.json
ARG PERSISTENT_PEERS=201cff36d13c6352acfc4a373b60e83211cd3102@bootstrap.southuk.azure.com:26656

ENV GENESISPATH="${GENESISPATH}"
ENV CHAINID="${CHAINID}"
ENV MONIKER="${MONIKER}"
ENV PERSISTENT_PEERS="${PERSISTENT_PEERS}"

#ENV LD_LIBRARY_PATH=/opt/sgxsdk/libsgx-enclave-common/:/opt/sgxsdk/lib64/

# Run secretd by default, omit entrypoint to ease using container with secretcli
ENTRYPOINT ["/bin/bash", "startup.sh"]
