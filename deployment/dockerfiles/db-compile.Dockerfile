FROM baiduxlab/sgx-rust:2004-1.1.3

### Install rocksdb

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    libgflags-dev \
    libsnappy-dev \
    zlib1g-dev \
    libbz2-dev \
    liblz4-dev \
    libzstd-dev

RUN git clone https://github.com/facebook/rocksdb.git

WORKDIR rocksdb

RUN git checkout v6.24.2
RUN export CXXFLAGS='-Wno-error=deprecated-copy -Wno-error=pessimizing-move -Wno-error=class-memaccess'
RUN make shared_lib -j 24
RUN make install-shared INSTALL_PATH=/usr