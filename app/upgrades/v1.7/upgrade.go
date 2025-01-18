package v1_7

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"

	"cosmossdk.io/log"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
	reg "github.com/scrtlabs/SecretNetwork/x/registration"
)

const upgradeName = "v1.7"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          upgradeName,
	CreateUpgradeHandler: createUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{Added: []string{icacontrollertypes.StoreKey}},
}

func createUpgradeHandler(mm *module.Manager, keepers *keepers.SecretAppKeepers, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		logger := log.NewLogger(os.Stderr)
		logger.Info(` _    _ _____   _____ _____            _____  ______ `)
		logger.Info(`| |  | |  __ \ / ____|  __ \     /\   |  __ \|  ____|`)
		logger.Info(`| |  | | |__) | |  __| |__) |   /  \  | |  | | |__   `)
		logger.Info(`| |  | |  ___/| | |_ |  _  /   / /\ \ | |  | |  __|  `)
		logger.Info(`| |__| | |    | |__| | | \ \  / ____ \| |__| | |____ `)
		logger.Info(` \____/|_|     \_____|_|  \_\/_/    \_\_____/|______|`)

		logger.Info(fmt.Sprintf("Running module migrations for %s...", upgradeName))

		seedb64, err := os.ReadFile(reg.SeedPath)
		if err != nil {
			return nil, err
		}

		seed, err := base64.StdEncoding.DecodeString(string(seedb64))
		if err != nil {
			return nil, err
		}

		// ecall_initialize_node will rewrite the new key to types.NodeExchMasterKeyPath
		masterKeyB64, err := os.ReadFile(reg.NodeExchMasterKeyPath)
		if err != nil {
			return nil, err
		}

		cfg := reg.SeedConfig{
			EncryptedKey: hex.EncodeToString(seed),
			MasterKey:    string(masterKeyB64),
			Version:      reg.SeedConfigVersion,
		}

		cfgBytes, err := json.Marshal(&cfg)
		if err != nil {
			return nil, err
		}

		// Remove the compute dir part
		homeDir := filepath.Dir(keepers.ComputeKeeper.HomeDir[:len(keepers.ComputeKeeper.HomeDir)-1])

		seedFilePath := filepath.Join(homeDir, reg.SecretNodeCfgFolder, reg.SecretNodeSeedNewConfig)

		err = os.WriteFile(seedFilePath, cfgBytes, 0o600)
		if err != nil {
			return nil, err
		}

		ioMasterKeyB64, err := os.ReadFile(reg.IoExchMasterKeyPath)
		if err != nil {
			return nil, err
		}

		masterKeyBz, err := base64.StdEncoding.DecodeString(string(masterKeyB64))
		if err != nil {
			return nil, err
		}

		ioMasterKeyBz, err := base64.StdEncoding.DecodeString(string(ioMasterKeyB64))
		if err != nil {
			return nil, err
		}

		masterKey := reg.MasterKey{Bytes: masterKeyBz}
		ioMasterKey := reg.MasterKey{Bytes: ioMasterKeyBz}

		keepers.RegKeeper.SetMasterKey(sdk.UnwrapSDKContext(ctx), ioMasterKey, reg.MasterIoKeyId)
		keepers.RegKeeper.SetMasterKey(sdk.UnwrapSDKContext(ctx), masterKey, reg.MasterNodeKeyId)

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
