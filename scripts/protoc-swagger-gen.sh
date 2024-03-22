#!/usr/bin/env bash

set -eo pipefail

mkdir -p ./tmp-swagger-gen
# Note: need to add ./third_party/proto/cosmos ./third_party/proto/ibc to a list of proto dirs when imports are fixed
proto_dirs=$(find ./proto ./third_party/proto -path -prune -o -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
for dir in $proto_dirs; do
  # generate swagger files (filter query files)
  query_file=$(find "${dir}" -maxdepth 1 \( -name 'query.proto' -o -name 'service.proto' \))
  if [[ ! -z "$query_file" ]]; then
    buf generate --template buf.gen.swagger.yaml $query_file
  fi
done

jq 'del(.definitions["cosmos.tx.v1beta1.ModeInfo.Multi"].properties.mode_infos.items["$ref"])' ./tmp-swagger-gen/cosmos/tx/v1beta1/service.swagger.json > ./tmp-swagger-gen/cosmos/tx/v1beta1/fixed-service.swagger.json
# # Tag everything as "gRPC Gateway API"
perl -i -pe 's/"(Query|Service)"/"gRPC Gateway API"/' $(find ./tmp-swagger-gen -name '*.swagger.json' -print0 | xargs -0)

(
  cd ./client/docs

  # Generate config.json
  # There's some operationIds naming collision, for sake of automation we're
  # giving all of them a unique name
  find ../../tmp-swagger-gen -name 'query.swagger.json' -o -name 'fixed-service.swagger.json' | 
    sort |
    awk '{print "{\"url\":\""$1"\",\"operationIds\":{\"rename\":{\"Params\":\""$1"Params\",\"Pool\":\""$1"Pool\",\"DelegatorValidators\":\""$1"DelegatorValidators\",\"UpgradedConsensusState\":\""$1"UpgradedConsensusState\"}}}"}' |
    jq -s '{swagger:"2.0","info":{"title":"Secret Network","description":"A REST interface for queries and transactions","version":"'"${CHAIN_VERSION}"'"},apis:.} | .apis += [{"url":"./swagger_legacy.yaml","dereference":{"circular":"ignore"}}]' > ./config.json

  # Derive openapi & swagger from config.json
  # yarn install
  # yarn combine
  # yarn convert
  # yarn build
)

cd ./client/docs
mkdir -p swagger-ui
# # combine swagger files
# # uses nodejs package `swagger-combine`.
# # all the individual swagger files need to be configured in `config.json` for merging
swagger-combine ./config.json -o ./swagger-ui/swagger.yaml -f yaml --continueOnConflictingPaths true --includeDefinitions true

cd ../..
# # clean swagger files
rm -rf ./tmp-swagger-gen
