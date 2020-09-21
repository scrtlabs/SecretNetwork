# Base image
FROM rust-go-base-image AS build-env-rust-go

# Final image
FROM cashmaney/enigma-sgx-base

# wasmi-sgx-test script requirements
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    #### Base utilities ####
    jq \
    wget \
    curl \
    bash-completion && \
    rm -rf /var/lib/apt/lists/*


RUN echo "source /etc/profile.d/bash_completion.sh" >> ~/.bashrc

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

COPY x/compute/internal/keeper/testdata/erc20.wasm erc20.wasm

COPY deployment/ci/wasmi-sgx-test.sh .
COPY deployment/ci/bootstrap_init.sh .
COPY deployment/ci/node_init.sh .
COPY deployment/ci/startup.sh .
COPY deployment/ci/node_key.json .

RUN chmod +x /usr/bin/secretd
RUN chmod +x /usr/bin/secretcli
RUN chmod +x wasmi-sgx-test.sh
RUN chmod +x bootstrap_init.sh
RUN chmod +x startup.sh
RUN chmod +x node_init.sh


RUN mkdir -p /root/.secretd/.compute/
RUN mkdir -p /root/.sgx_secrets/
RUN mkdir -p /root/.secretd/.node/

# Enable autocomplete
RUN secretcli completion > /root/secretcli_completion
RUN secretd completion > /root/secretd_completion

RUN echo 'source /root/secretd_completion' >> ~/.bashrc
RUN echo 'source /root/secretcli_completion' >> ~/.bashrc

#ENV LD_LIBRARY_PATH=/opt/sgxsdk/libsgx-enclave-common/:/opt/sgxsdk/lib64/

# Run secretd by default, omit entrypoint to ease using container with secretcli
ENTRYPOINT ["/bin/bash", "startup.sh"]
