FROM ubuntu:focal as runtime_base

LABEL maintainer=enigmampc

# SGX version parameters
ARG SGX_VERSION=2.12.100.3
ARG OS_REVESION=focal1

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    #### Base utilities ####
    logrotate \
    gdebi \
    wget \
    libprotobuf17 \
    #### SGX installer dependencies ####
    g++ make libcurl4 libssl1.1 && \
    rm -rf /var/lib/apt/lists/*


#RUN wget -O /tmp/libprotobuf10_3.0.0-9_amd64.deb http://ftp.br.debian.org/debian/pool/main/p/protobuf/libprotobuf10_3.0.0-9_amd64.deb
#RUN yes | gdebi /tmp/libprotobuf10_3.0.0-9_amd64.deb

WORKDIR /root

# Must create /etc/init or enclave-common install will fail
RUN mkdir /etc/init && \
    mkdir sgx


##### Install SGX Binaries ######
ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/debian_pkgs/libs/libsgx-enclave-common/libsgx-enclave-common_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/debian_pkgs/libs/libsgx-urts/libsgx-urts_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/debian_pkgs/libs/libsgx-uae-service/libsgx-uae-service_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/debian_pkgs/libs/libsgx-quote-ex/libsgx-quote-ex_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/debian_pkgs/libs/libsgx-epid/libsgx-epid_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/
ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/debian_pkgs/libs/libsgx-launch/libsgx-launch_${SGX_VERSION}-${OS_REVESION}_amd64.deb ./sgx/


RUN dpkg -i ./sgx/libsgx-enclave-common_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-urts_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-launch_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-epid_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-quote-ex_${SGX_VERSION}-${OS_REVESION}_amd64.deb && \
    dpkg -i ./sgx/libsgx-uae-service_${SGX_VERSION}-${OS_REVESION}_amd64.deb

ADD https://download.01.org/intel-sgx/sgx-linux/2.12/distro/ubuntu20.04-server/sgx_linux_x64_sdk_${SGX_VERSION}.bin ./sgx/
# ADD https://download.01.org/intel-sgx/sgx-linux/2.9.1/distro/ubuntu18.04-server/sgx_linux_x64_driver_2.6.0_95eaa6f.bin ./sgx/

RUN chmod +x ./sgx/sgx_linux_x64_sdk_${SGX_VERSION}.bin
RUN echo -e 'no\n/opt' | ./sgx/sgx_linux_x64_sdk_${SGX_VERSION}.bin && \
    echo 'source /opt/sgxsdk/environment' >> /root/.bashrc && \
    rm -rf ./sgx/*

ENV LD_LIBRARY_PATH=/opt/sgxsdk/libsgx-enclave-common/

#RUN SGX_DEBUG=0 SGX_MODE=HW SGX_PRERELEASE=1 make