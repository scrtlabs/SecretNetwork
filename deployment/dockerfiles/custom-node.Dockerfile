# Final image
FROM enigmampc/enigma-sgx-base:2004-1.1.3

# wasmi-sgx-test script requirements
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    #### Base utilities ####
    jq \
    openssl \
    curl \
    wget \
    bash-completion && \
    rm -rf /var/lib/apt/lists/*

RUN echo "source /etc/profile.d/bash_completion.sh" >> ~/.bashrc

ARG SGX_MODE=SW
ENV SGX_MODE=${SGX_MODE}

ARG SECRET_NODE_TYPE=BOOTSTRAP
ENV SECRET_NODE_TYPE=${SECRET_NODE_TYPE}

ENV PKG_CONFIG_PATH=""
ENV LD_LIBRARY_PATH=""

ENV SCRT_ENCLAVE_DIR=/usr/lib/

# workaround because paths seem kind of messed up
RUN cp /opt/sgxsdk/lib64/libsgx_urts_sim.so /usr/lib/libsgx_urts_sim.so
RUN cp /opt/sgxsdk/lib64/libsgx_uae_service_sim.so /usr/lib/libsgx_uae_service_sim.so

# Install ca-certificates
WORKDIR /root

# Copy over binaries from the local directory
COPY ./go-cosmwasm/api/libgo_cosmwasm.so.x /usr/lib/libgo_cosmwasm.so
COPY ./cosmwasm/enclaves/execute/librust_cosmwasm_enclave.signed.so.x /usr/lib/librust_cosmwasm_enclave.signed.so
COPY ./secretd /usr/bin/secretd
COPY ./secretcli /usr/bin/secretcli

COPY deployment/docker/local/bootstrap_init.sh .
COPY deployment/docker/local/node_init.sh .
COPY deployment/docker/startup.sh .
COPY deployment/docker/node_key.json .

RUN chmod +x /usr/bin/secretd
RUN chmod +x bootstrap_init.sh
RUN chmod +x node_init.sh
RUN chmod +x startup.sh

# Enable autocomplete
RUN secretcli completion > /root/secretcli_completion
RUN secretd completion > /root/secretd_completion

RUN echo 'source /root/secretd_completion' >> ~/.bashrc
RUN echo 'source /root/secretcli_completion' >> ~/.bashrc

RUN mkdir -p /root/.secretd/.compute/
RUN mkdir -p /root/.sgx_secrets/
RUN mkdir -p /root/.secretd/.node/

#COPY deployment/docker/bootstrap/config.toml /root/.secretd/config/config-cli.toml
#
#COPY x/compute/internal/keeper/testdata/erc20.wasm /root/erc20.wasm
#COPY deployment/docker/sanity-test.sh /root/
#
#RUN chmod +x /root/sanity-test.sh

####### Node parameters
ARG MONIKER=default
ARG CHAINID=secret-testnet-1
ARG GENESISPATH=https://raw.githubusercontent.com/enigmampc/SecretNetwork/master/secret-testnet-genesis.json
ARG PERSISTENT_PEERS=201cff36d13c6352acfc4a373b60e83211cd3102@bootstrap.southuk.azure.com:26656

ENV GENESISPATH="${GENESISPATH}"
ENV CHAINID="${CHAINID}"
ENV MONIKER="${MONIKER}"
ENV PERSISTENT_PEERS="${PERSISTENT_PEERS}"

#ENV LD_LIBRARY_PATH=/opt/sgxsdk/libsgx-enclave-common/:/opt/sgxsdk/lib64/

# Run secretd by default, omit entrypoint to ease using container with secretcli
ENTRYPOINT ["/bin/bash", "startup.sh"]
