FROM baiduxlab/sgx-rust:2004-1.1.3

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

RUN --mount=type=secret,id=SPID,dst=/run/secrets/spid.txt cat /run/secrets/spid.txt > /enclave-test/cosmwasm/enclaves/execute/spid.txt
RUN --mount=type=secret,id=API_KEY,dst=/run/secrets/api_key.txt cat /run/secrets/api_key.txt > /enclave-test/cosmwasm/enclaves/execute/api_key.txt

COPY deployment/ci/enclave-test.sh .
RUN chmod +x enclave-test.sh

ENTRYPOINT ["/bin/bash", "enclave-test.sh"]
