## Secret Node Azure Template


This folder contains both the marketplace template (`createUiDefinition.json` 
and `mainTemplate.json`) and the quickstart template (`azuredeploy.json`).

Not going to go into the composition and definition of these, but you can read more on the 
[azure documentation pages](https://docs.microsoft.com/en-us/azure/marketplace/partner-center-portal/create-new-azure-apps-offer)  

### Deploy quickstart template

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

### Registration Service

To automate node registration, I deployed a simple service that automatically performs the on-chain tranactions
for convenience. The testnet service is available at `reg-snet-test1.uksouth.azurecontainer.io:8081`

In mainnet we may also want to offer this service, since it really doesn't cost much, and basic rate-limiting and
certificate validation should make sure it can't be abused.

With the available API being a single GET request at `/register` with a required `cert` parameter as a url-encoded base64 string
of the certificate created by the `secretd init-enclave` command

Example:

`GET http://reg-snet-test1.uksouth.azurecontainer.io:8081/register?cert=<attestation_cert.der>`

With the expected response being `200` (and the txhash, though this is unused) on success and `500` on any error.

#### Manual setup

Since we only need one of these per network, the register command that is run by the service is a simple
call to the `secretcli`. This means we need some manual setup (to add SCRT to the service, and configure the node):

`secretcli config chain-id <chain-id>`

`secretcli config trust-node true`

`secretcli config output json`

`secretcli keys add a --recover`

The RPC address is set by the environment variable `RPC_URL` or defaults to the address of the bootstrap node used in
internal testnet.

While it shouldn't make a difference, make sure `SGX_MODE` environment variable is properly set as well
