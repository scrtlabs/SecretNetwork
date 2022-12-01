FROM ubuntu:focal as runtime_base

LABEL maintainer=enigmampc

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    #### Base utilities ####
    logrotate \
    #### SGX installer dependencies ####
    g++ make libcurl4 libssl1.1 libprotobuf17 && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /root

# Must create /etc/init or enclave-common install will fail
RUN mkdir /etc/init && \
    mkdir sgx


# SGX version parameters
ARG SGX_MAJOR_VERSION=2.12
ARG SGX_MINOR_VERSION=.100.3
ARG OS_REVESION=focal1
ARG OS_NAME=ubuntu20.04-server

ARG SGX_VERSION=${SGX_MAJOR_VERSION}${SGX_MINOR_VERSION}

# todo: figure out what we need and what not jesus christ
##### Install SGX Binaries ######
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/libs/libsgx-enclave-common/libsgx-enclave-common_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/libs/libsgx-urts/libsgx-urts_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/utils/sgx-aesm-service/sgx-aesm-service_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/utils/libsgx-aesm-epid-plugin/libsgx-aesm-epid-plugin_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/utils/libsgx-aesm-quote-ex-plugin/libsgx-aesm-quote-ex-plugin_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/utils/libsgx-aesm-launch-plugin/libsgx-aesm-launch-plugin_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/utils/libsgx-ae-epid/libsgx-ae-epid_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/utils/libsgx-ae-pce/libsgx-ae-pce_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/utils/libsgx-aesm-pce-plugin/libsgx-aesm-pce-plugin_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/utils/libsgx-aesm-ecdsa-plugin/libsgx-aesm-ecdsa-plugin_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/utils/libsgx-ae-le/libsgx-ae-le_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/

ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/libs/libsgx-pce-logic/libsgx-pce-logic_1.9.100.3-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/libs/libsgx-qe3-logic/libsgx-qe3-logic_1.9.100.3-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/debian_pkgs/libs/libsgx-ae-qe3/libsgx-ae-qe3_1.9.100.3-${OS_REVESION}_amd64.deb ./sgx/


RUN dpkg -i ./sgx/libsgx-enclave-common_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-urts_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/sgx-aesm-service_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    # plugin dependencies
    dpkg -i ./sgx/libsgx-ae-epid_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-ae-pce_${SGX_VERSION}-${OS_REVESION}_amd64.deb&& \
    dpkg -i ./sgx/libsgx-ae-le_${SGX_VERSION}-${OS_REVESION}_amd64.deb&& \
    dpkg -i ./sgx/libsgx-pce-logic_1.9.100.3-${OS_REVESION}_amd64.deb&& \
    dpkg -i ./sgx/libsgx-ae-qe3_1.9.100.3-${OS_REVESION}_amd64.deb&& \
    dpkg -i ./sgx/libsgx-qe3-logic_1.9.100.3-${OS_REVESION}_amd64.deb&& \
    # AESM plugins
    dpkg -i ./sgx/libsgx-aesm-pce-plugin_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-aesm-ecdsa-plugin_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-aesm-epid-plugin_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-aesm-quote-ex-plugin_${SGX_VERSION}-${OS_REVESION}_amd64.deb&& \
    dpkg -i ./sgx/libsgx-aesm-launch-plugin_${SGX_VERSION}-${OS_REVESION}_amd64.deb

ADD https://download.01.org/intel-sgx/sgx-linux/${SGX_MAJOR_VERSION}/distro/${OS_NAME}/sgx_linux_x64_sdk_${SGX_VERSION}.bin ./sgx/
# ADD https://download.01.org/intel-sgx/sgx-linux/2.9.1/distro/${OS_NAME}/sgx_linux_x64_driver_2.6.0_95eaa6f.bin ./sgx/

RUN chmod +x ./sgx/sgx_linux_x64_sdk_${SGX_VERSION}.bin
RUN echo -e 'no\n/opt' | ./sgx/sgx_linux_x64_sdk_${SGX_VERSION}.bin && \
    echo 'source /opt/sgxsdk/environment' >> /root/.bashrc && \
    rm -rf ./sgx/*

USER aesmd
WORKDIR /opt/intel/sgx-aesm-service/aesm/
ENV LD_LIBRARY_PATH=.
CMD ./aesm_service --no-daemon
