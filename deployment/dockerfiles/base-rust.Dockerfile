FROM baiduxlab/sgx-rust:2004-1.1.3 AS build-env-rust-go

ENV PATH="/root/.cargo/bin:$PATH"

#RUN wget -q https://github.com/WebAssembly/wabt/releases/download/1.0.20/wabt-1.0.20-ubuntu.tar.gz && \
#    tar -xf wabt-1.0.20-ubuntu.tar.gz wabt-1.0.20/bin/wat2wasm wabt-1.0.20/bin/wasm2wat && \
#    mv wabt-1.0.20/bin/wat2wasm wabt-1.0.20/bin/wasm2wat /bin && \
#    chmod +x /bin/wat2wasm /bin/wasm2wat && \
#    rm -f wabt-1.0.20-ubuntu.tar.gz

# Set working directory for the build
WORKDIR /go/src/github.com/enigmampc/SecretNetwork/

ARG BUILD_VERSION="v0.0.0"
ARG SGX_MODE=SW
ARG FEATURES
ARG FEATURES_U

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