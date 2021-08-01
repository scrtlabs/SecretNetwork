module github.com/enigmampc/SecretNetwork

go 1.15

require (
	github.com/armon/go-metrics v0.3.9 // indirect
	github.com/btcsuite/btcd v0.22.0-beta // indirect
	github.com/coinbase/rosetta-sdk-go v0.6.10 // indirect
	//github.com/CosmWasm/wasmd v0.11.1 // indirect
	//github.com/DataDog/zstd v1.4.5 // indirect
	github.com/cosmos/cosmos-sdk v0.43.0-rc2
	github.com/cosmos/ibc-go v1.0.0-rc3
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	//github.com/dgraph-io/badger/v2 v2.2007.2 // indirect
	//github.com/dgraph-io/ristretto v0.0.3 // indirect
	//github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/gogo/protobuf v1.3.3
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.2
	github.com/google/gofuzz v1.1.1-0.20200604201612-c04b05f3adfa
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/hdevalence/ed25519consensus v0.0.0-20210204194344-59a8610d2b87 // indirect
	github.com/improbable-eng/grpc-web v0.14.0 // indirect
	github.com/jhump/protoreflect v1.8.2 // indirect
	github.com/mattn/go-isatty v0.0.13 // indirect
	//github.com/jteeuwen/go-bindata v3.0.7+incompatible
	github.com/miscreant/miscreant.go v0.0.0-20200214223636-26d376326b75
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.29.0 // indirect
	github.com/rakyll/statik v0.1.7
	github.com/rs/zerolog v1.23.0
	github.com/spf13/cast v1.3.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.0
	github.com/stretchr/testify v1.7.0
	github.com/tendermint/tendermint v0.34.11
	github.com/tendermint/tm-db v0.6.4
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c
	google.golang.org/grpc v1.38.0
	gopkg.in/yaml.v2 v2.4.0 // indirect
	nhooyr.io/websocket v1.8.6 // indirect
)

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

replace google.golang.org/grpc => google.golang.org/grpc v1.33.2

// replace github.com/cosmos/cosmos-sdk v0.43.0-rc2 => github.com/enigmampc/cosmos-sdk secret-0.43

replace github.com/cosmos/cosmos-sdk v0.43.0-rc2 => github.com/enigmampc/cosmos-sdk v0.0.0-20210801171644-dc0a78417f77

//replace github.com/cosmos/cosmos-sdk v0.43.0-rc2 => github.com/enigmampc/cosmos-sdk v0.0.0-cc218f2182fb
