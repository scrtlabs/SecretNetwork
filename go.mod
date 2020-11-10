module github.com/enigmampc/SecretNetwork

go 1.15

require (
	github.com/CosmWasm/wasmd v0.11.1 // indirect
	github.com/DataDog/zstd v1.4.5 // indirect
	github.com/cosmos/cosmos-sdk v0.40.0-rc1
	github.com/cosmos/iavl v0.15.0-rc4 // indirect
	github.com/dgraph-io/badger/v2 v2.2007.2 // indirect
	github.com/dgraph-io/ristretto v0.0.3 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.2
	github.com/golang/snappy v0.0.2 // indirect
	github.com/google/gofuzz v1.0.0
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.15.2
	github.com/jteeuwen/go-bindata v3.0.7+incompatible // indirect
	github.com/miscreant/miscreant.go v0.0.0-20200214223636-26d376326b75
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.14.0
	github.com/rakyll/statik v0.1.7
	github.com/spf13/cast v1.3.1
	github.com/spf13/cobra v1.1.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	github.com/tendermint/tendermint v0.34.0-rc5.0.20201028154430-ad4f54e9b211
	github.com/tendermint/tm-db v0.6.2
	golang.org/x/crypto v0.0.0-20201012173705-84dcc777aaee
	golang.org/x/net v0.0.0-20200930145003-4acb6c075d10 // indirect
	google.golang.org/genproto v0.0.0-20201014134559-03b6142f0dc9
	google.golang.org/grpc v1.33.0
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.2-alpha.regen.4

replace google.golang.org/grpc => google.golang.org/grpc v1.33.1

replace github.com/tendermint/tendermint => /home/toml/Dev/tendermint
