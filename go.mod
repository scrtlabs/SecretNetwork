module github.com/enigmampc/SecretNetwork

go 1.15

require (
	//github.com/CosmWasm/wasmd v0.11.1 // indirect
	//github.com/DataDog/zstd v1.4.5 // indirect
	github.com/cosmos/cosmos-sdk v0.44.3
	//	github.com/enigmampc/cosmos-sdk v0.9.1-scrt
	github.com/cosmos/ibc-go v1.1.2
	//github.com/dgraph-io/badger/v2 v2.2007.2 // indirect
	//github.com/dgraph-io/ristretto v0.0.3 // indirect
	//github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/gogo/protobuf v1.3.3
	github.com/golang/protobuf v1.5.2
	github.com/google/gofuzz v1.1.1-0.20200604201612-c04b05f3adfa
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	//github.com/jteeuwen/go-bindata v3.0.7+incompatible
	github.com/miscreant/miscreant.go v0.0.0-20200214223636-26d376326b75
	github.com/pkg/errors v0.9.1
	github.com/rakyll/statik v0.1.7
	github.com/rs/zerolog v1.23.0
	github.com/spf13/cast v1.3.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/tendermint/tendermint v0.34.14
	github.com/tendermint/tm-db v0.6.4
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1
)

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

replace google.golang.org/grpc => google.golang.org/grpc v1.33.2

// replace github.com/cosmos/cosmos-sdk v0.43.0-rc2 => github.com/enigmampc/cosmos-sdk secret-0.43

replace github.com/cosmos/cosmos-sdk v0.44.3 => github.com/scrtlabs/cosmos-sdk v0.44.4-0.20211117100413-27366ad17d3a

//replace github.com/cosmos/cosmos-sdk v0.43.0-rc2 => github.com/enigmampc/cosmos-sdk v0.0.0-cc218f2182fb
