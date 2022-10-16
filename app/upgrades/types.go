package upgrades

import (
	store "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	abci "github.com/tendermint/tendermint/abci/types"
)

// BaseAppParamManager defines an interrace that BaseApp is expected to fullfil
// that allows upgrade handlers to modify BaseApp parameters.
type BaseAppParamManager interface {
	GetConsensusParams(ctx sdk.Context) *abci.ConsensusParams
	StoreConsensusParams(ctx sdk.Context, cp *abci.ConsensusParams)
}

// Upgrade defines a struct containing necessary fields that a SoftwareUpgradeProposal
// must have written, in order for the state migration to go smoothly.
// An upgrade must implement this struct, and then set it in the app.go.
// The app.go will then define the handler.
type Upgrade struct {
	// Upgrade version name, for the upgrade handler, e.g. `v1.4`
	UpgradeName string

	// CreateUpgradeHandler defines the function that creates an upgrade handler
	// mm *module.Manager, computeModule *computetypes.AppModule, configurator module.Configurator
	CreateUpgradeHandler func(*module.Manager, *keepers.SecretAppKeepers, module.Configurator) upgradetypes.UpgradeHandler

	// Store upgrades, should be used for any new modules introduced, new modules deleted, or store names renamed.
	StoreUpgrades store.StoreUpgrades
}
