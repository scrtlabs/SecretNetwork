package main

import (
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/scrtlabs/SecretNetwork/x/compute"
)

// SecretAppConfig terra specify app config
type SecretAppConfig struct {
	serverconfig.Config

	WASMConfig compute.WasmConfig `mapstructure:"wasm"`
}

// initCometBFTConfig helps to override default CometBFT Config values.
// return cmtcfg.DefaultConfig if no custom configuration is required for the application.
func initCometBFTConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()

	// these values put a higher strain on node memory
	// cfg.P2P.MaxNumInboundPeers = 100
	// cfg.P2P.MaxNumOutboundPeers = 40

	return cfg
}

// initAppConfig helps to override default appConfig template and configs.
// return "", nil if no custom configuration is required for the application.
func initAppConfig() (string, interface{}) {
	// Optionally allow the chain developer to overwrite the SDK's default
	// server config.
	srvCfg := serverconfig.DefaultConfig()

	// The SDK's default minimum gas price is set to "" (empty value) inside
	// app.toml. If left empty by validators, the node will halt on startup.
	// However, the chain developer can set a default app.toml value for their
	// validators here.
	//
	// In summary:
	// - if you leave srvCfg.MinGasPrices = "", all validators MUST tweak their
	//   own app.toml config,
	// - if you set srvCfg.MinGasPrices non-empty, validators CAN tweak their
	//   own app.toml to override, or use this default value.
	//
	// In simapp, we set the min gas prices to 0.
	// Assaf: This changes the default if the config is not present at all
	// (E.g. a new config after a chain upgrade)
	srvCfg.MinGasPrices = "0.0125uscrt"
	srvCfg.API.Enable = true
	srvCfg.API.Swagger = true
	srvCfg.API.EnableUnsafeCORS = true
	srvCfg.GRPCWeb.Enable = true

	// defaulting this to false until we can verify it's amazballs
	srvCfg.GRPC.Concurrency = false

	secretAppConfig := SecretAppConfig{
		Config:     *srvCfg,
		WASMConfig: *compute.DefaultWasmConfig(),
	}
	secretAppConfig.Config.IAVLDisableFastNode = false

	secretAppConfig.WASMConfig.EnclaveCacheSize = 200

	secretAppTemplate := serverconfig.DefaultConfigTemplate + compute.DefaultConfigTemplate

	return secretAppTemplate, secretAppConfig
}
