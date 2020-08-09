#!/bin/bash
set -euv

source /opt/sgxsdk/environment
make enclave-tests
