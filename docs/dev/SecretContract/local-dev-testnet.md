---
title: Setup the Local Developer Testnet
---

# Setup the Local Developer Testnet
The developer blockchain is configured to run inside a docker container. Install [Docker](https://docs.docker.com/install/) for your environment (Mac, Windows, Linux).

Open a terminal window and change to your project directory.
Then start SecretNetwork, labelled _secretdev_ from here on:

```
$ docker run -it --rm \
 -p 26657:26657 -p 26656:26656 -p 1317:1317 \
 --name secretdev enigmampc/secret-network-bootstrap-sw:latest
```

**NOTE**: The _secretdev_ docker container can be stopped by CTRL+C

![](../../images/images/docker-run.png)

At this point you're running a local SecretNetwork full-node. Let's connect to the container so we can view and manage the secret keys:

**NOTE**: In a new terminal

```
docker exec -it secretdev /bin/bash
```

The local blockchain has a couple of keys setup for you (similar to accounts if you're familiar with Truffle Ganache). The keys are stored in the `test` keyring backend, which makes it easier for local development and testing.

```
secretcli keys list --keyring-backend test
```

![](../../images/images/secretcli_keys_list.png)

`exit` when you are done
