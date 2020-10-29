module github.com/enigmampc/SecretNetwork

go 1.15

require (
	github.com/btcsuite/btcd v0.21.0-beta
	github.com/btcsuite/btcutil v1.0.2
	github.com/cosmos/cosmos-sdk v0.40.0-rc1
	github.com/google/gofuzz v1.0.0
	github.com/gorilla/mux v1.8.0
	github.com/miscreant/miscreant.go v0.0.0-20200214223636-26d376326b75
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.14.0
	github.com/spf13/cobra v1.1.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	github.com/tendermint/go-amino v0.16.0
	github.com/tendermint/tendermint v0.34.0-rc5
	github.com/tendermint/tm-db v0.6.2
	golang.org/x/crypto v0.0.0-20201012173705-84dcc777aaee
	gopkg.in/yaml.v2 v2.3.0
//github.com/btcsuite/btcd v0.21.0-beta
//github.com/btcsuite/btcutil v1.0.2
//github.com/cosmos/cosmos-sdk v0.40.0-rc1
//github.com/ethereum/go-ethereum v1.9.22
//github.com/gorilla/mux v1.8.0
//github.com/pkg/errors v0.9.1
//github.com/rakyll/statik v0.1.7
//github.com/spf13/cast v1.3.1
//github.com/spf13/cobra v1.1.0
//github.com/tendermint/tendermint v0.34.0-rc5
//github.com/tendermint/tm-db v0.6.2
)

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.2-alpha.regen.4

replace google.golang.org/grpc => google.golang.org/grpc v1.33.1
