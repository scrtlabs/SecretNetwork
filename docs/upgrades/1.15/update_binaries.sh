#!/bin/bash
set -o xtrace

docker exec bootstrap bash -c 'rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin'
docker exec node bash -c 'rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin'
# update bootstrap
docker cp ./bin/secretd                               bootstrap:/tmp/upgrade-bin
docker cp ./bin/librust_cosmwasm_enclave.signed.so    bootstrap:/tmp/upgrade-bin
docker cp ./bin/libgo_cosmwasm.so                     bootstrap:/tmp/upgrade-bin
# update node
docker cp ./bin/secretd                               node:/tmp/upgrade-bin
docker cp ./bin/librust_cosmwasm_enclave.signed.so    node:/tmp/upgrade-bin
docker cp ./bin/libgo_cosmwasm.so                     node:/tmp/upgrade-bin
docker cp ./bin/librandom_api.so                      node:/tmp/upgrade-bin
docker cp ./bin/tendermint_enclave.signed.so          node:/tmp/upgrade-bin
# stop node's secretd
docker exec node bash -c 'pkill -9 secretd'
# copy over updated binaries
docker exec bootstrap bash -c 'cp /tmp/upgrade-bin/librust_cosmwasm_enclave.signed.so /usr/lib/'
docker exec bootstrap bash -c 'cp /tmp/upgrade-bin/libgo_cosmwasm.so /usr/lib/'
docker exec node bash -c 'cp /tmp/upgrade-bin/secretd /usr/bin/'
docker exec node bash -c 'cp /tmp/upgrade-bin/librust_cosmwasm_enclave.signed.so /usr/lib/'
docker exec node bash -c 'cp /tmp/upgrade-bin/libgo_cosmwasm.so /usr/lib/'

# prepare a tmp dir to store validator's private key
rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin
docker cp bootstrap:/root/.secretd/config/priv_validator_key.json /tmp/upgrade-bin/.
docker cp /tmp/upgrade-bin/priv_validator_key.json node:/root/.secretd/config/priv_validator_key.json
