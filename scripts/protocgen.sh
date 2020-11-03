#!/usr/bin/env bash

set -eo pipefail

### Compute Module ###

project_dir=x/compute/internal/types/
cosmos_sdk_dir=$(go list -f "{{ .Dir }}" -m github.com/cosmos/cosmos-sdk)
# Generate Go types from protobuf
protoc \
  -I=. \
  -I="$cosmos_sdk_dir/third_party/proto" \
  -I="$cosmos_sdk_dir/proto" \
  --gocosmos_out=Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types,plugins=interfacetype+grpc,paths=source_relative:. \
  $(find "${project_dir}" -maxdepth 1 -name '*.proto')

# Generate gRPC gateway (*.pb.gw.go in respective modules) files
protoc \
  -I=. \
  -I="$cosmos_sdk_dir/third_party/proto" \
  -I="$cosmos_sdk_dir/proto" \
  --grpc-gateway_out .\
  --grpc-gateway_opt logtostderr=true \
  --grpc-gateway_opt paths=source_relative \
  $(find "${project_dir}" -maxdepth 1 -name '*.proto')

### Registration Module ###

project_dir=x/registration/internal/types/
cosmos_sdk_dir=$(go list -f "{{ .Dir }}" -m github.com/cosmos/cosmos-sdk)
# Generate Go types from protobuf
protoc \
  -I=. \
  -I="$cosmos_sdk_dir/third_party/proto" \
  -I="$cosmos_sdk_dir/proto" \
  --gocosmos_out=Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types,plugins=interfacetype+grpc,paths=source_relative:. \
  $(find "${project_dir}" -maxdepth 1 -name '*.proto')

# Generate gRPC gateway (*.pb.gw.go in respective modules) files
protoc \
  -I=. \
  -I="$cosmos_sdk_dir/third_party/proto" \
  -I="$cosmos_sdk_dir/proto" \
  --grpc-gateway_out .\
  --grpc-gateway_opt logtostderr=true \
  --grpc-gateway_opt paths=source_relative \
  $(find "${project_dir}" -maxdepth 1 -name '*.proto')

### Registration Module - Remote Attestation Types ###

project_dir=x/registration/remote_attestation/
cosmos_sdk_dir=$(go list -f "{{ .Dir }}" -m github.com/cosmos/cosmos-sdk)
# Generate Go types from protobuf
protoc \
  -I=. \
  -I="$cosmos_sdk_dir/third_party/proto" \
  -I="$cosmos_sdk_dir/proto" \
  --gocosmos_out=Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types,plugins=interfacetype+grpc,paths=source_relative:. \
  $(find "${project_dir}" -maxdepth 1 -name '*.proto')

# Generate gRPC gateway (*.pb.gw.go in respective modules) files
protoc \
  -I=. \
  -I="$cosmos_sdk_dir/third_party/proto" \
  -I="$cosmos_sdk_dir/proto" \
  --grpc-gateway_out .\
  --grpc-gateway_opt logtostderr=true \
  --grpc-gateway_opt paths=source_relative \
  $(find "${project_dir}" -maxdepth 1 -name '*.proto')