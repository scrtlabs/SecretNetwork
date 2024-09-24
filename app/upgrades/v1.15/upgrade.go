package v1_15

import (
	"context"
	"fmt"
	"os"

	"cosmossdk.io/log"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibcclienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types" //nolint:staticcheck
	ibcconntypes "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibcswitchtypes "github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"

	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
)

const upgradeName = "v1.15"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          upgradeName,
	CreateUpgradeHandler: createUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added: []string{
			icacontrollertypes.StoreKey,
			consensusparamtypes.StoreKey,
			crisistypes.StoreKey,
		},
	},
}

func createUpgradeHandler(mm *module.Manager, appKeepers *keepers.SecretAppKeepers, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		logger := log.NewLogger(os.Stderr)
		logger.Info(` _    _ _____   _____ _____            _____  ______ `)
		logger.Info(`| |  | |  __ \ / ____|  __ \     /\   |  __ \|  ____|`)
		logger.Info(`| |  | | |__) | |  __| |__) |   /  \  | |  | | |__   `)
		logger.Info(`| |  | |  ___/| | |_ |  _  /   / /\ \ | |  | |  __|  `)
		logger.Info(`| |__| | |    | |__| | | \ \  / ____ \| |__| | |____ `)
		logger.Info(` \____/|_|     \_____|_|  \_\/_/    \_\_____/|______|`)

		// Set param key table for params module migration
		for _, subspace := range appKeepers.ParamsKeeper.GetSubspaces() {
			subspace := subspace

			var keyTable paramstypes.KeyTable
			switch subspace.Name() {
			case authtypes.ModuleName:
				keyTable = authtypes.ParamKeyTable() //nolint:staticcheck
			case banktypes.ModuleName:
				keyTable = banktypes.ParamKeyTable() //nolint:staticcheck
			case stakingtypes.ModuleName:
				keyTable = stakingtypes.ParamKeyTable() //nolint:staticcheck
			case minttypes.ModuleName:
				keyTable = minttypes.ParamKeyTable() //nolint:staticcheck
			case distrtypes.ModuleName:
				keyTable = distrtypes.ParamKeyTable() //nolint:staticcheck
			case slashingtypes.ModuleName:
				keyTable = slashingtypes.ParamKeyTable() //nolint:staticcheck
			case govtypes.ModuleName:
				keyTable = govv1.ParamKeyTable() //nolint:staticcheck
			case ibcswitchtypes.ModuleName:
				keyTable = ibcswitchtypes.ParamKeyTable()
			case crisistypes.ModuleName:
				keyTable = crisistypes.ParamKeyTable() //nolint:staticcheck
			case ibcexported.ModuleName:
				keyTable = ibcclienttypes.ParamKeyTable()
				keyTable.RegisterParamSet(&ibcconntypes.Params{})
			case ibctransfertypes.ModuleName:
				keyTable = ibctransfertypes.ParamKeyTable()
			case icacontrollertypes.SubModuleName:
				keyTable = icacontrollertypes.ParamKeyTable()
			case icahosttypes.SubModuleName:
				keyTable = icahosttypes.ParamKeyTable()
			default:
				continue
			}

			if !subspace.HasKeyTable() {
				subspace.WithKeyTable(keyTable)
			}
		}

		baseAppLegacySS := appKeepers.ParamsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramstypes.ConsensusParamsKeyTable())
		if err := baseapp.MigrateParams(
			sdk.UnwrapSDKContext(ctx),
			baseAppLegacySS,
			appKeepers.ConsensusParamsKeeper.ParamsStore); err != nil {
			logger.Error(fmt.Sprintf("ERROR: %s %s", upgradeName, err))
			return module.VersionMap{}, nil
		}
		logger.Info(fmt.Sprintf("Running module migrations for %s...", upgradeName))
		logger.Debug(fmt.Sprintf("Version map: %v", vm))
		m, e := mm.RunMigrations(ctx, configurator, vm)
		if e != nil {
			logger.Error(fmt.Sprintf("[x] Run migration error: %s", e))
		} else {
			logger.Info(fmt.Sprintf("[v] Successful migration run for %s", upgradeName))
		}
		return m, e
	}
}
