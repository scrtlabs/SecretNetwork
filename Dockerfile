# Simple usage with a mounted data directory:
# > docker build -t secret .
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.secretd:/root/.secretd -v ~/.secretcli:/root/.secretcli secret secretd init
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.secretd:/root/.secretd -v ~/.secretcli:/root/.secretcli secret secretd start
FROM golang:alpine AS build-env

# Set up dependencies
ENV PACKAGES curl make git libc-dev bash gcc linux-headers eudev-dev python

# Install minimum necessary dependencies, build Cosmos SDK, remove packages
RUN apk add $PACKAGES

# Set working directory for the build
WORKDIR /go/src/github.com/enigmampc/secretnetwork

# Add source files
COPY . .

RUN make build_local

# Final image
FROM alpine:edge

# Install ca-certificates
RUN apk add --update ca-certificates
WORKDIR /root

# Run enigmad by default, omit entrypoint to ease using container with enigmacli
# CMD ["/bin/bash"]

# Copy over binaries from the build-env
COPY --from=build-env /go/src/github.com/enigmampc/secretnetwork/secretd /usr/bin/secretd
COPY --from=build-env  /go/src/github.com/enigmampc/secretnetwork/secretcli /usr/bin/secretcli

COPY ./packaging_docker/docker_start.sh .

RUN chmod +x /usr/bin/enigmad
RUN chmod +x /usr/bin/enigmacli
RUN chmod +x docker_start.sh .
# Run enigmad by default, omit entrypoint to ease using container with enigmacli
#CMD ["/root/enigmad"]

####### STAGE 1 -- build core
ARG moniker=default
ARG chainid=secret-1
ARG genesis_path=https://raw.githubusercontent.com/enigmampc/SecretNetwork/master/secret-1-genesis.json
ARG persistent_peers=201cff36d13c6352acfc4a373b60e83211cd3102@bootstrap.mainnet.enigma.co:26656

ENV GENESISPATH=$genesis_path
ENV CHAINID=$chainid
ENV MONIKER=$moniker
ENV PERSISTENT_PEERS=$persistent_peers

ENTRYPOINT ["/bin/bash", "docker_start.sh"]
