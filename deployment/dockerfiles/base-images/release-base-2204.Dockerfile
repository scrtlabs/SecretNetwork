FROM ubuntu:jammy as runtime_base

LABEL maintainer=enigmampc

# SGX version parameters
ARG SDK_VERSION=2.18.1
ARG SGX_VERSION=2.18.101.1
ARG PSW_VERSION=2.18.101.1-jammy1
ARG OS_REVESION=jammy1

WORKDIR /root

# Must create /etc/init or enclave-common install will fail
RUN mkdir /etc/init && \
    mkdir sgx


RUN apt-get update && \
    apt-get install -y gnupg2 apt-transport-https ca-certificates curl software-properties-common make g++ libcurl4 libssl3 && \
    curl -fsSL https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | apt-key add - && \
    add-apt-repository "deb https://download.01.org/intel-sgx/sgx_repo/ubuntu jammy main" && \
    apt-get update && \
    apt-get install -y \
        libsgx-aesm-launch-plugin=$PSW_VERSION \
        libsgx-enclave-common=$PSW_VERSION \
        libsgx-epid=$PSW_VERSION \
        libsgx-launch=$PSW_VERSION \
        libsgx-quote-ex=$PSW_VERSION \
        libsgx-uae-service=$PSW_VERSION \
        libsgx-urts=$PSW_VERSION && \
    rm -rf /var/lib/apt/lists/* && \
    rm -rf /var/cache/apt/archives/* && \
    mkdir /var/run/aesmd


ADD https://download.01.org/intel-sgx/sgx-linux/${SDK_VERSION}/distro/ubuntu22.04-server/sgx_linux_x64_sdk_${SGX_VERSION}.bin ./sgx/

RUN chmod +x ./sgx/sgx_linux_x64_sdk_${SGX_VERSION}.bin
RUN echo -e 'no\n/opt' | ./sgx/sgx_linux_x64_sdk_${SGX_VERSION}.bin && \
    echo 'source /opt/sgxsdk/environment' >> /root/.bashrc && \
    rm -rf ./sgx/*

ENV LD_LIBRARY_PATH=/opt/sgxsdk/libsgx-enclave-common/
