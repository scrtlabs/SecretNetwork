#!/usr/bin/env bash

set -eo pipefail

mkdir -p ./tmp-swagger-gen
proto_dirs=$(find ./proto ./third_party/proto/cosmos ./third_party/proto/ibc -path -prune -o -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
for dir in $proto_dirs; do

  # generate swagger files (filter query files)
  query_file=$(find "${dir}" -maxdepth 1 \( -name 'query.proto' -o -name 'service.proto' \))
  # remove autocli, because it contains circular refs
  if [[ ! -z "$query_file" && "$query_file" != *autocli* && "$query_file" != *gov/v1beta1* ]]; then
    # 1. Get buf from https://github.com/bufbuild/buf/releases/tag/v1.0.0-rc12
    # Note that v1.0.0-rc12 is the last version with the "buf protoc" subcommand
    # 2. Get swagger protoc plugin with `go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger@v1.16.0`

    buf generate --template buf.gen.swagger.yaml $query_file
  fi
done

# Tag everything as "gRPC Gateway API"
perl -i -pe 's/"(Query|Service)"/"gRPC Gateway API"/' $(find ./tmp-swagger-gen -name '*.swagger.json' -print0 | xargs -0)

(
  cd ./client/docs

  # Generate config.json
  # There's some operationIds naming collision, for sake of automation we're
  # giving all of them a unique name
  find ../../tmp-swagger-gen -name 'query.swagger.json' | 
    sort |
    awk '{print "{\"url\":\""$1"\",\"operationIds\":{\"rename\":{\"Params\":\""$1"Params\",\"Pool\":\""$1"Pool\",\"DelegatorValidators\":\""$1"DelegatorValidators\",\"UpgradedConsensusState\":\""$1"UpgradedConsensusState\",\"Accounts\":\""$1"Accounts\",\"Account\":\""$1"Account\",\"Proposal\":\""$1"Proposal\",\"Proposals\":\""$1"Proposals\",\"Deposits\":\""$1"Deposits\",\"Deposit\":\""$1"Deposit\",\"TallyResult\":\""$1"TallyResult\",\"Votes\":\""$1"Votes\",\"Vote\":\""$1"Vote\",\"Balance\":\""$1"Balance\",\"Code\":\""$1"Code\"}}}"}' |
    jq -s '{swagger:"2.0","info":{"title":"Secret Network","description":"A REST interface for queries and transactions","version":"'"${CHAIN_VERSION}"'"},apis:.}' > ./config.json

  # Derive openapi & swagger from config.json
  yarn install
  yarn combine
  yarn convert
  yarn build
)

# clean swagger tmp files
rm -rf ./tmp-swagger-gen
