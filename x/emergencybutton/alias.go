package emergencybutton

import (
	"github.com/scrtlabs/SecretNetwork/x/emergencybutton/keeper"
	"github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"
)

const (
	ModuleName   = types.ModuleName
	StoreKey     = types.StoreKey
	TStoreKey    = types.TStoreKey
	QuerierRoute = types.QuerierRoute
	RouterKey    = types.QuerierRoute
)

var (
	NewKeeper        = keeper.NewKeeper
	NewMsgServerImpl = keeper.NewMsgServerImpl
)

type (
	GenesisState = types.GenesisState
	Keeper       = keeper.Keeper
)
