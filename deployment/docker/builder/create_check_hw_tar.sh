#!/bin/bash

mkdir -p /build/check-hw/
cp ./check-hw /build/check-hw/
cp ./check_hw_enclave.so /build/check-hw/
cp ./README.md /build/check-hw/

cd /build/
tar -czf check_hw_"$VERSION".tar.gz check-hw
