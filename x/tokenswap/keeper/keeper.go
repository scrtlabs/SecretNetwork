package keeper

import (
	"fmt"
	"math"
	"strings"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/enigmampc/Enigmachain/x/tokenswap/types"

	"github.com/cosmos/cosmos-sdk/codec"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Keeper maintains the link to data storage and exposes getter/setter methods for the various parts of the state machine
type Keeper struct {
	cdc          *codec.Codec // The wire codec for binary encoding/decoding.
	storeKey     sdk.StoreKey // Unexposed key to access store from sdk.Context
	supplyKeeper types.SupplyKeeper
}

// NewKeeper creates new instances of the oracle Keeper
func NewKeeper(cdc *codec.Codec, storeKey sdk.StoreKey, supplyKeeper types.SupplyKeeper) Keeper {
	return Keeper{
		cdc:          cdc,
		storeKey:     storeKey,
		supplyKeeper: supplyKeeper,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// ProcessTokenSwapRequest processes a claim that has just completed successfully with consensus
func (k Keeper) ProcessTokenSwapRequest(ctx sdk.Context, ethereumTxHash string, ethereumSender string, receiver sdk.AccAddress, amountENG float64) error {
	// Convert ENG to uSCRT
	amountUscrt := int64(math.Ceil(amountENG * 1e6))
	amountUscrtCoins := sdk.NewCoins(sdk.NewCoin("uscrt", sdk.NewInt(amountUscrt)))

	// Lowercase ethereumTxHash as this is our indexed field
	ethereumTxHashLowercase := strings.ToLower(ethereumTxHash)
	tokenSwap := types.NewTokenSwap(ethereumTxHashLowercase, ethereumSender, receiver, amountUscrtCoins)

	// Mint new uSCRTs
	err := k.supplyKeeper.MintCoins(ctx, types.ModuleName, tokenSwap.AmountUSCRT)
	if err != nil {
		return err
	}

	// Transfer new funds to receiver
	err = k.supplyKeeper.SendCoinsFromModuleToAccount(
		ctx, types.ModuleName, tokenSwap.Receiver, tokenSwap.AmountUSCRT,
	)
	if err != nil {
		panic(err)
	}

	// Store the token swap request in our state
	// We need this to verify we process each request only once
	store := ctx.KVStore(k.storeKey)
	store.Set([]byte(tokenSwap.EthereumTxHash), k.cdc.MustMarshalBinaryBare(tokenSwap))

	return nil
}

// GetPastTokenSwapRequest retrives a past token swap request
func (k Keeper) GetPastTokenSwapRequest(ctx sdk.Context, ethereumTxHash string) (types.TokenSwap, error) {
	store := ctx.KVStore(k.storeKey)

	// Lowercase ethereumTxHash as this is our indexed field
	ethereumTxHashLowercase := strings.ToLower(ethereumTxHash)

	if !store.Has([]byte(ethereumTxHashLowercase)) {
		return types.TokenSwap{}, sdkerrors.Wrap(
			sdkerrors.ErrUnknownRequest,
			"Unknown Ethereum tx hash "+ethereumTxHash)

	}

	bz := store.Get([]byte(ethereumTxHashLowercase))
	var tokenSwap types.TokenSwap
	k.cdc.MustUnmarshalBinaryBare(bz, &tokenSwap)

	return tokenSwap, nil
}
