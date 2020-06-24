#!/usr/bin/env bash

# shellcheck disable=SC2174
mkdir -p -m 777 /tmp/aesmd
chmod -R -f 777 /tmp/aesmd || sudo chmod -R -f 777 /tmp/aesmd || true

docker-compose up aesmd bootstrap -d

docker-compose up node