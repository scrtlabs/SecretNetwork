# Simple usage with a mounted data directory:
# > docker build -t gaia .
# > docker run -it -p 46657:46657 -p 46656:46656 -v ~/.gaiad:/root/.gaiad -v ~/.gaiacli:/root/.gaiacli gaia gaiad init
# > docker run -it -p 46657:46657 -p 46656:46656 -v ~/.gaiad:/root/.gaiad -v ~/.gaiacli:/root/.gaiacli gaia gaiad start
FROM golang:alpine AS build-env

# Set up dependencies
ENV PACKAGES curl make git libc-dev bash gcc linux-headers eudev-dev python

# Install minimum necessary dependencies, build Cosmos SDK, remove packages
RUN apk add $PACKAGES

# Set working directory for the build
WORKDIR /go/src/github.com/enigmampc/enigmachain

# Add source files
COPY . .

RUN make build_local

# Final image
FROM alpine:edge

# Install ca-certificates
RUN apk add --update ca-certificates
WORKDIR /root

# Run gaiad by default, omit entrypoint to ease using container with gaiacli
# CMD ["/bin/bash"]

# Copy over binaries from the build-env
COPY --from=build-env /go/src/github.com/enigmampc/enigmachain/enigmad /usr/bin/enigmad
COPY --from=build-env  /go/src/github.com/enigmampc/enigmachain/enigmacli /usr/bin/enigmacli

COPY ./packaging_docker/docker_start.sh .

RUN chmod +x /usr/bin/enigmad
RUN chmod +x /usr/bin/enigmacli
RUN chmod +x docker_start.sh .
# Run gaiad by default, omit entrypoint to ease using container with gaiacli
#CMD ["/root/enigmad"]

####### STAGE 1 -- build core
ARG moniker=default
ARG chainid=enigma-1
ARG genesis_path=https://raw.githubusercontent.com/enigmampc/EnigmaBlockchain/master/enigma-1-genesis.json
ARG persistent_peers=201cff36d13c6352acfc4a373b60e83211cd3102@bootstrap.mainnet.enigma.co:26656

ENV GENESISPATH=$genesis_path
ENV CHAINID=$chainid
ENV MONIKER=$moniker
ENV PERSISTENT_PEERS=$persistent_peers

ENTRYPOINT ["/bin/ash", "docker_start.sh"]