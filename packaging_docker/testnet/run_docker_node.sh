#!/usr/bin/env bash

mkdir -p -m 777 /tmp/aesmd
chmod -R -f 777 /tmp/aesmd || sudo chmod -R -f 777 /tmp/aesmd || true

docker-compose -f docker-compose.testnet.node.yaml up -d