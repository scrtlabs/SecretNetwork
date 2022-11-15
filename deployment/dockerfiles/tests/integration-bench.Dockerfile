# This dockerfile contains tests that require a full network to run, and require a running node that is connected to the network

FROM ghcr.io/scrtlabs/compile-contracts:1.6.0

COPY deployment/ci/query-load-test query-load-test

WORKDIR query-load-test

RUN npm install

ENTRYPOINT ["node", "test.js"]