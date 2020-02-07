package tokenswap

import (
	"github.com/enigmampc/Enigmachain/x/tokenswap/keeper"
	"github.com/enigmampc/Enigmachain/x/tokenswap/types"
)

const (
	TokenSwap    = types.TokenSwap
	ModuleName   = types.ModuleName
	StoreKey     = types.StoreKey
	QuerierRoute = types.QuerierRoute
	RouterKey    = types.RouterKey
)

var (
	// functions aliases
	NewKeeper                         = keeper.NewKeeper
	NewQuerier                        = keeper.NewQuerier
	NewEthBridgeClaim                 = types.NewEthBridgeClaim
	NewOracleClaimContent             = types.NewOracleClaimContent
	CreateOracleClaimFromEthClaim     = types.CreateOracleClaimFromEthClaim
	CreateEthClaimFromOracleString    = types.CreateEthClaimFromOracleString
	CreateOracleClaimFromOracleString = types.CreateOracleClaimFromOracleString
	RegisterCodec                     = types.RegisterCodec
	NewEthereumAddress                = types.NewEthereumAddress
	NewMsgCreateEthBridgeClaim        = types.NewMsgCreateEthBridgeClaim
	MapOracleClaimsToEthBridgeClaims  = types.MapOracleClaimsToEthBridgeClaims
	NewQueryEthProphecyParams         = types.NewGetTokenSwapParams
	NewQueryEthProphecyResponse       = types.NewQueryEthProphecyResponse
)

type (
	Keeper                   = keeper.Keeper
	EthBridgeClaim           = types.EthBridgeClaim
	OracleClaimContent       = types.OracleClaimContent
	EthereumAddress          = types.EthereumAddress
	MsgCreateEthBridgeClaim  = types.MsgCreateEthBridgeClaim
	MsgBurn                  = types.MsgBurn
	MsgLock                  = types.MsgLock
	QueryEthProphecyParams   = types.QueryEthProphecyParams
	QueryEthProphecyResponse = types.QueryEthProphecyResponse
)
