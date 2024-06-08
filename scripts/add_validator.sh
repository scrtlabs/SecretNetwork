#!/usr/bin/bash

VALIDATOR_PK=$(secretd tendermint show-validator | tr -d '\\')
echo ${VALIDATOR_PK}

jq -n \
	--arg vpk ${VALIDATOR_PK} \
	'
{
  "pubkey": $vpk,
  "amount": "1000000uscrt",
  "moniker": "my-moniker",
  "website": "https://myweb.site",
  "security": "monitoring@scrtlabs.com",
  "details": "scrt2 validator",
  "commission-rate": "0.10",
  "commission-max-rate": "0.20",
  "commission-max-change-rate": "0.01",
  "min-self-delegation": "1"
}' > ./validator.json.tmp

cat ./validator.json.tmp | sed 's/\\"/"/g' > validator.json

# Note: to test redelegations all keys must be present on all nodes
secretd tx staking create-validator ./validator.json --from a --fees 5000uscrt


