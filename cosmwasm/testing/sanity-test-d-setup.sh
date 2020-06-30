#!/bin/bash

#!/bin/bash

# Use this script to run secretd with debugger
# Put this in .vscode/launch.json:
# {
#   "version": "0.2.0",
#   "configurations": [
#     {
#       "name": "Go",
#       "type": "go",
#       "request": "launch",
#       "mode": "auto",
#       "cwd": "${workspaceFolder}",
#       "program": "${workspaceFolder}/cmd/secretd",
#       "env": { "SGX_MODE": "SW" },
#       "args": ["start", "--bootstrap"]
#     }
#   ]
# }
# And then:
# 1. Build secretcli and secretd `SGX_MODE=SW make build-linux`
# 2. Init the node: `SGX_MODE=SW cosmwasm/testing/sanity-test-d-setup.sh `
# 3. Launch vscode in debug mode (you can set breakpoints in secretd go code)
# 4. Run the tests with secretcli: `SGX_MODE=SW cosmwasm/testing/sanity-test-only-cli.sh`

set -euvx

# init the node
rm -rf ./.sgx_secrets
mkdir -p ./.sgx_secrets

rm -rf ~/.enigma*

./secretd init banana --chain-id enigma-testnet
perl -i -pe 's/"stake"/"uscrt"/g' ~/.secretd/config/genesis.json
./secretcli config keyring-backend test
echo "cost member exercise evoke isolate gift cattle move bundle assume spell face balance lesson resemble orange bench surge now unhappy potato dress number acid" |
    ./secretcli keys add a --recover
./secretd add-genesis-account "$(./secretcli keys show -a a)" 1000000000000uscrt
./secretd gentx --name a --keyring-backend test --amount 1000000uscrt
./secretd collect-gentxs
./secretd validate-genesis

./secretd init-bootstrap ./node-master-cert.der ./io-master-cert.der

./secretd validate-genesis
