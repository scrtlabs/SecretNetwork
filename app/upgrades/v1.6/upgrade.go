package v1_7

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	store "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
	reg "github.com/scrtlabs/SecretNetwork/x/registration"
	"github.com/scrtlabs/SecretNetwork/x/registration/remote_attestation"
)

const upgradeName = "v1.6"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          upgradeName,
	CreateUpgradeHandler: createUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
}

func createUpgradeHandler(mm *module.Manager, keepers *keepers.SecretAppKeepers, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info(` _    _ _____   _____ _____            _____  ______ `)
		ctx.Logger().Info(`| |  | |  __ \ / ____|  __ \     /\   |  __ \|  ____|`)
		ctx.Logger().Info(`| |  | | |__) | |  __| |__) |   /  \  | |  | | |__   `)
		ctx.Logger().Info(`| |  | |  ___/| | |_ |  _  /   / /\ \ | |  | |  __|  `)
		ctx.Logger().Info(`| |__| | |    | |__| | | \ \  / ____ \| |__| | |____ `)
		ctx.Logger().Info(` \____/|_|     \_____|_|  \_\/_/    \_\_____/|______|`)

		ctx.Logger().Info("Running module migrations for v1.6...")

		seedb64, err := os.ReadFile(reg.SeedPath)
		if err != nil {
			return nil, err
		}

		seed, err := base64.StdEncoding.DecodeString(string(seedb64))
		if err != nil {
			return nil, err
		}

		// ecall_initialize_node will rewrite the new key to types.NodeExchMasterKeyPath
		masterKeyBz, err := os.ReadFile(reg.NodeExchMasterKeyPath)
		if err != nil {
			return nil, err
		}

		cfg := reg.SeedConfig{
			EncryptedKey: hex.EncodeToString(seed),
			MasterKey:    string(masterKeyBz),
		}

		cfgBytes, err := json.Marshal(&cfg)
		if err != nil {
			return nil, err
		}

		seedFilePath := filepath.Join(keepers.ComputeKeeper.HomeDir, reg.SecretNodeCfgFolder, reg.SecretNodeSeedConfig)
		prevSeedFileBz, err := os.ReadFile(seedFilePath)
		if err != nil {
			return nil, err
		}

		err = os.WriteFile(seedFilePath, cfgBytes, 0o600)
		if err != nil {
			return nil, err
		}

		ioMasterKeyBz, err := os.ReadFile(reg.IoExchMasterKeyPath)
		if err != nil {
			return nil, err
		}

		masterKey := reg.MasterKey{Bytes: masterKeyBz}
		ioMasterKey := reg.MasterKey{Bytes: ioMasterKeyBz}

		keepers.RegKeeper.SetMasterKey(ctx, masterKey, reg.MasterIoKeyId)
		keepers.RegKeeper.SetMasterKey(ctx, ioMasterKey, reg.MasterNodeKeyId)

		var prevSeedCfg reg.LegacySeedConfig
		err = json.Unmarshal(prevSeedFileBz, &prevSeedCfg)
		if err != nil {
			return nil, err
		}

		cert, err := base64.StdEncoding.DecodeString(prevSeedCfg.MasterCert)
		if err != nil {
			return nil, err
		}

		regInfo := reg.RegistrationNodeInfo{
			Certificate:   remote_attestation.Certificate(cert),
			EncryptedSeed: seed,
		}

		keepers.RegKeeper.SetRegistrationInfo(ctx, regInfo)

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
