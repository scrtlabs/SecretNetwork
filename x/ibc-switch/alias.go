package ibc_switch

import (
	"github.com/scrtlabs/SecretNetwork/x/ibc-switch/keeper"
	"github.com/scrtlabs/SecretNetwork/x/ibc-switch/types"
)

const (
	ModuleName   = types.ModuleName
	StoreKey     = types.StoreKey
	TStoreKey    = types.TStoreKey
	QuerierRoute = types.QuerierRoute
	RouterKey    = types.QuerierRoute
)

var (
	NewKeeper = keeper.NewKeeper
)

type (
	GenesisState = types.GenesisState
	Keeper       = keeper.Keeper
)
