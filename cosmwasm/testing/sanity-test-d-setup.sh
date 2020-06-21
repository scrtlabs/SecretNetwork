#!/bin/bash

#!/bin/bash

# Use this script to run enigmad with debugger
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
#       "program": "${workspaceFolder}/cmd/enigmad",
#       "env": { "SGX_MODE": "SW" },
#       "args": ["start", "--bootstrap"]
#     }
#   ]
# }
# And then:
# 1. Build enigmacli and enigmad `SGX_MODE=SW make build_linux`
# 2. Init the node: `SGX_MODE=SW cosmwasm/testing/sanity-test-d-setup.sh `
# 3. Launch vscode in debug mode (you can set breakpoints in enigmad go code)
# 4. Run the tests with enigmacli: `SGX_MODE=SW cosmwasm/testing/sanity-test-only-cli.sh`

set -euvx

# init the node
rm -rf ./.sgx_secrets
mkdir -p ./.sgx_secrets

rm -rf ~/.enigma*

./enigmad init banana --chain-id enigma-testnet
perl -i -pe 's/"stake"/"uscrt"/g' ~/.enigmad/config/genesis.json
./enigmacli config keyring-backend test
echo "cost member exercise evoke isolate gift cattle move bundle assume spell face balance lesson resemble orange bench surge now unhappy potato dress number acid" |
    ./enigmacli keys add a --recover
./enigmad add-genesis-account "$(./enigmacli keys show -a a)" 1000000000000uscrt
./enigmad gentx --name a --keyring-backend test --amount 1000000uscrt
./enigmad collect-gentxs
./enigmad validate-genesis

./enigmad init-bootstrap ./node-master-cert.der ./io-master-cert.der

./enigmad validate-genesis
