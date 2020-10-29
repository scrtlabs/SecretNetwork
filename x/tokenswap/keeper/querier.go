package keeper

import (
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/enigmampc/SecretNetwork/x/tokenswap/types"
)

// query endpoints supported by the tokenswap Querier
const (
	GetTokenSwap = "get"
)

// NewQuerier is the module level router for state queries
func NewQuerier(keeper Keeper, cdc *codec.Codec) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) ([]byte, error) {
		switch path[0] {
		case types.GetTokenSwapRoute:
			return getTokenSwapRequest(ctx, cdc, req, keeper)
		default:
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "unknown tokenswap query endpoint "+path[0])
		}
	}
}

func getTokenSwapRequest(ctx sdk.Context, cdc *codec.Codec, req abci.RequestQuery, keeper Keeper) ([]byte, error) {
	var params types.GetTokenSwapParams

	if err := cdc.UnmarshalJSON(req.Data, &params); err != nil {
		return nil, sdkerrors.Wrap(err, fmt.Sprintf("failed to parse params from '%s'", string(req.Data)))
	}

	tokenSwap, err := keeper.GetPastTokenSwapRequest(ctx, params.EthereumTxHash)
	if err != nil {
		return nil, err
	}

	return cdc.MarshalJSONIndent(tokenSwap, "", "  ")
}
