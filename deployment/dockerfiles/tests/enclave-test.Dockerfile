FROM ghcr.io/scrtlabs/sgx-rust:2004-1.1.6

RUN add-apt-repository -r "deb https://download.01.org/intel-sgx/sgx_repo/ubuntu focal main" && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    #### Base utilities ####
    clang && \
    rm -rf /var/lib/apt/lists/*

ENV PATH="/root/.cargo/bin:$PATH"
ARG SGX_MODE=SW
ENV SGX_MODE=${SGX_MODE}
ARG FEATURES="test"
ENV FEATURES=${FEATURES}
ENV PKG_CONFIG_PATH=""
ENV LD_LIBRARY_PATH=""
#ENV MITIGATION_CVE_2020_0551=LOAD

# Set working directory for the build
WORKDIR /enclave-test/

# Add source files
COPY third_party third_party
COPY cosmwasm/ cosmwasm/
COPY Makefile Makefile

COPY rust-toolchain rust-toolchain
RUN rustup component add rust-src
RUN cargo install xargo --version 0.3.25

COPY deployment/ci/enclave-test.sh .
RUN chmod +x enclave-test.sh

ENTRYPOINT ["/bin/bash", "enclave-test.sh"]
