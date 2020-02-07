package tokenswap

import (
	"github.com/enigmampc/Enigmachain/x/tokenswap/keeper"
	"github.com/enigmampc/Enigmachain/x/tokenswap/types"
)

const (
	ModuleName   = types.ModuleName
	RouterKey    = types.RouterKey
	StoreKey     = types.StoreKey
	QuerierRoute = types.QuerierRoute
)

// functions aliases
var (
	NewKeeper             = keeper.NewKeeper
	NewQuerier            = keeper.NewQuerier
	NewTokenSwap          = types.NewTokenSwap
	NewMsgTokenSwap       = types.NewMsgTokenSwap
	NewGetTokenSwapParams = types.NewGetTokenSwapParams
)

type (
	Keeper                 = keeper.Keeper
	TokenSwap              = types.TokenSwap
	MsgTokenSwap           = types.MsgTokenSwap
	QueryEthProphecyParams = types.GetTokenSwapParams
)
