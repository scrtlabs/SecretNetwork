FROM ubuntu:focal as runtime_base

LABEL maintainer=enigmampc

# SGX version parameters
ARG SDK_VERSION=2.20
ARG SGX_VERSION=2.20.100.4
ARG PSW_VERSION=2.20.100.4-focal1
ARG OS_REVESION=focal1
ARG DCAP_VERSION=1.17.100.4-focal1
#RUN apt-get update && \
#    apt-get install -y --no-install-recommends \
#    #### Base utilities ####
#    logrotate \
#    gdebi \
#    wget \
#    # libprotobuf17 \
#    # gnupg \
#    #### SGX installer dependencies ####
#    g++ make libcurl4 libssl1.1 && \
#    rm -rf /var/lib/apt/lists/*


#RUN wget -O /tmp/libprotobuf10_3.0.0-9_amd64.deb http://ftp.br.debian.org/debian/pool/main/p/protobuf/libprotobuf10_3.0.0-9_amd64.deb
#RUN yes | gdebi /tmp/libprotobuf10_3.0.0-9_amd64.deb

WORKDIR /root

# Must create /etc/init or enclave-common install will fail
RUN mkdir /etc/init && \
    mkdir sgx


RUN apt-get update && \
    apt-get install -y gnupg2 apt-transport-https ca-certificates curl software-properties-common make g++ libcurl4 libssl3 && \
    curl -fsSL https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | apt-key add - && \
    add-apt-repository "deb https://download.01.org/intel-sgx/sgx_repo/ubuntu focal main" && \
    apt-get update && \
    apt-get install -y \
        libsgx-aesm-launch-plugin=$PSW_VERSION \
        libsgx-enclave-common=$PSW_VERSION \
        libsgx-epid=$PSW_VERSION \
        libsgx-launch=$PSW_VERSION \
        libsgx-quote-ex=$PSW_VERSION \
        libsgx-uae-service=$PSW_VERSION \
        libsgx-qe3-logic=$DCAP_VERSION \
        libsgx-pce-logic=$DCAP_VERSION \
        libsgx-aesm-ecdsa-plugin=$PSW_VERSION \
        libsgx-aesm-pce-plugin=$PSW_VERSION \
        libsgx-dcap-ql=$DCAP_VERSION \
        libsgx-dcap-quote-verify=$DCAP_VERSION \
        libsgx-dcap-default-qpl=$DCAP_VERSION \
        libsgx-urts=$PSW_VERSION && \
    rm -rf /var/lib/apt/lists/* && \
    rm -rf /var/cache/apt/archives/* && \
    mkdir /var/run/aesmd
#        libsgx-headers=$VERSION \
 #        libsgx-ae-epid=$VERSION \
 #        libsgx-ae-le=$VERSION \
 #        libsgx-ae-pce=$VERSION \
 #        libsgx-aesm-ecdsa-plugin=$VERSION \
 #        libsgx-aesm-epid-plugin=$VERSION \
 #        libsgx-aesm-launch-plugin=$VERSION \
 #        libsgx-aesm-pce-plugin=$VERSION \
 #        libsgx-aesm-quote-ex-plugin=$VERSION \
 #        libsgx-enclave-common=$VERSION \
 #        libsgx-enclave-common-dev=$VERSION \
 #        libsgx-epid=$VERSION \
 #        libsgx-epid-dev=$VERSION \
 #        libsgx-launch=$VERSION \
 #        libsgx-launch-dev=$VERSION \
 #        libsgx-quote-ex=$VERSION \
 #        libsgx-quote-ex-dev=$VERSION \
 #        libsgx-uae-service=$VERSION \
 #        libsgx-urts=$VERSION \
 #        sgx-aesm-service=$VERSION \

# ENTRYPOINT ["/bin/bash"]

# RUN apt-get update


#RUN apt-get install libsgx-epid libsgx-quote-ex libsgx-dcap-ql
#
ADD https://download.01.org/intel-sgx/sgx-linux/${SDK_VERSION}/distro/ubuntu20.04-server/sgx_linux_x64_sdk_${SGX_VERSION}.bin ./sgx/
# ADD https://download.01.org/intel-sgx/sgx-linux/${SDK_VERSION}/distro/ubuntu20.04-server/sgx_linux_x64_sdk_${SGX_VERSION}.bin ./sgx/
## ADD https://download.01.org/intel-sgx/sgx-linux/2.9.1/distro/ubuntu18.04-server/sgx_linux_x64_driver_2.6.0_95eaa6f.bin ./sgx/
##
RUN chmod +x ./sgx/sgx_linux_x64_sdk_${SGX_VERSION}.bin
RUN echo -e 'no\n/opt' | ./sgx/sgx_linux_x64_sdk_${SGX_VERSION}.bin && \
    echo 'source /opt/sgxsdk/environment' >> /root/.bashrc && \
    rm -rf ./sgx/*
##
ENV LD_LIBRARY_PATH=/opt/sgxsdk/libsgx-enclave-common/
#
##RUN SGX_DEBUG=0 SGX_MODE=HW SGX_PRERELEASE=1 make
