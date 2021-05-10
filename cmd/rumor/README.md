To build rumor as a local node you want to first run `make build-linux` to build all the
rust binaries once. I do this as a separate step to make iterative development on the Go
code faster.

Then, you can build the local dockers by running:
```shell
make build_rumor build-custom-dev-image build-custom-dev-image-rumor
```

Once built, you can start the local environment with a botstrap, node, and rumor by running:
```shell
docker-compose -f deployment/docker/docker-compose-rumor.yaml up
```
(chain-id `enigma-pub-testnet-3`)

This will open the following ports to the host machine:

* 26657 - RPC port for the node
* 8080 - LCD port for rumor
