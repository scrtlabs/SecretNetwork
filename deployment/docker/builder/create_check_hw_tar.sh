#!/bin/bash

mkdir -p /build/check-hw/
cp ./check-hw/check-hw /build/check-hw/
cp ./check-hw/check_hw_enclave.so /build/check-hw/
cp ./check-hw/check_hw_enclave_testnet.so /build/check-hw/
cp ./check-hw/README.md /build/check-hw/

cd /build/
tar -czf check_hw_"$VERSION".tar.gz check-hw
