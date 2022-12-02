# Azure Image Guide


## Requirements

* An Azure account

## General info

This folder contains templates which are used to create an Azure marketplace offering of a 1-click secret node.
Possibly in the future we will add more complex architectures, but at the moment it is just a single node. 

To make things 1-clicky and simple the node setup and registration is performed automatically, with all you need to do is configure the parameters of your node.

There are 2 variants in this folder - the marketplace template and the quickstart template. The quickstart template is basically a DIY version of the marketplace image which you
can customize or deploy yourself via the azure command-line.

Now lets go over how to do that, and how to use the image once it's deployed.

## Installation - Quickstart Template

Download this directory.

Using Azure CLI login to enigmampc using 

`az login`

Then, create a resource group for the new secret node

`az group create --name <resource-group-name> --location "UK South"` 

Note: Only UK south, East US and Central Canada are available for SGX machines

Finally, deploy the secret node using:
`az deployment group create --resource-group <resource-group-name> --template-file azuredeploy.json`

(or `az group deployment create --resource-group <resource-group-name> --template-file azuredeploy.json` on newer az cli versions)

By default the node will try to connect to our testnet node/registration service, so configure the parameters
accordingly to what you want

Note: When asked to enter a `adminPasswordOrKey` You can enter either your SSH public key, or a password. _However_, only using an SSH key is properly tested. So use a password at your own risk.

## Installation - Azure Marketplace

__Not available publicly yet. Will update this section once it is released__

## Usage

The actual node runs in a docker image on the machine that is created. However, it will come with some handy aliases to help you use it easily.

### SSH into your machine

```ssh <adminUsernam>@<vmDnsName>.<group_region>.cloudapp.azure.com```

(you can also find this string in the output of the group deployment command under the key `sshCommand`)

### Commands

`secretd`, `secretcli` - will work as you would normally expect

`show-node-id` - will print out the p2p address of the node
`show-validator` - will print out the validator consensus public key, which is used when creating a validator

`stop-secret-node` - will stop the node
`start-secret-node` - will start the node

### Debugging

After creating the machine a healthy status of the node will have 2 containers active:

```docker ps```

```
CONTAINER ID        IMAGE                                      COMMAND                  CREATED             STATUS                    PORTS                                  NAMES
bf9ba8dd0802        enigmampc/secret-network-node:pubtestnet   "/bin/bash startup.sh"   13 minutes ago      Up 13 minutes (healthy)   0.0.0.0:26656-26657->26656-26657/tcp   secret-node_node_1
2405b23aa1bd        enigmampc/aesm                             "/bin/sh -c './aesm_…"   13 minutes ago      Up 13 minutes                                                    secret-node_aesm_1
```

You can see the logs of the node by checking the docker logs of the node container:

```docker logs secret-node_node_1```

If you want to debug/do other stuff with your node you can exec into the actual node using

```docker exec -it secret-node_node_1 /bin/bash```

