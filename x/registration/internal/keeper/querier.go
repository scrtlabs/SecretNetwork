package keeper

import (
	"encoding/hex"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	abci "github.com/tendermint/tendermint/abci/types"
)

const (
	QueryEncryptedSeed = "seed"
)

// controls error output on querier - set true when testing/debugging
const debug = false

// NewQuerier creates a new querier
func NewQuerier(keeper Keeper) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) ([]byte, error) {
		switch path[0] {
		case QueryEncryptedSeed:
			return queryEncryptedSeed(ctx, path[1], req, keeper)
		default:
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "unknown data query endpoint")
		}
	}
}

func queryEncryptedSeed(ctx sdk.Context, pubkey string, req abci.RequestQuery, keeper Keeper) ([]byte, error) {
	fmt.Println("queryEncryptedSeed")

	pubkeyBytes, err := hex.DecodeString(pubkey)
	if err != nil {
		return nil, err
	}

	seed := keeper.getRegistrationInfo(ctx, pubkeyBytes).EncryptedSeed
	if seed == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownAddress, "Node has not been authenticated yet")
	}

	return seed, nil
}
