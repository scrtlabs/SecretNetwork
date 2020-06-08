package tokenswap

import (
	"github.com/enigmampc/EnigmaBlockchain/x/tokenswap/keeper"
	"github.com/enigmampc/EnigmaBlockchain/x/tokenswap/types"
)

const (
	DefaultParamspace = types.DefaultParamspace
	ModuleName        = types.ModuleName
	RouterKey         = types.RouterKey
	StoreKey          = types.StoreKey
	QuerierRoute      = types.QuerierRoute
)

// functions aliases
var (
	ModuleCdc           = types.ModuleCdc
	RegisterCodec       = types.RegisterCodec
	NewKeeper           = keeper.NewKeeper
	NewQuerier          = keeper.NewQuerier
	DefaultGenesisState = types.DefaultGenesisState
	NewGenesisState     = types.NewGenesisState
	// NewTokenSwap          = types.NewS
	// NewMsgTokenSwap       = types.NewMsgSwapRequest
	// NewGetTokenSwapParams = types.NewGetTokenSwapParams
)

type (
	GenesisState           = types.GenesisState
	SwapKeeper             = keeper.Keeper
	SupplyKeeper           = types.SupplyKeeper
	AccountKeeper          = types.AccountKeeper
	SwapRecord             = types.TokenSwapRecord
	MsgSwapRequest         = types.MsgSwapRequest
	QueryEthProphecyParams = types.GetTokenSwapParams
)
