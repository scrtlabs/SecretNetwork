package keeper

import (
	"context"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/scrtlabs/SecretNetwork/x/mint/types"
)

// Keeper of the mint store
type Keeper struct {
	cdc              codec.BinaryCodec
	storeService     storetypes.KVStoreService
	paramSpace       paramtypes.Subspace
	stakingKeeper    types.StakingKeeper
	bankKeeper       types.BankKeeper
	feeCollectorName string
	authority        string
}

// NewKeeper creates a new mint Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService storetypes.KVStoreService,
	sk types.StakingKeeper,
	ak types.AccountKeeper,
	bk types.BankKeeper,
	feeCollectorName string,
	authority string,
) Keeper {
	return Keeper{
		cdc:              cdc,
		storeService:     storeService,
		stakingKeeper:    sk,
		bankKeeper:       bk,
		feeCollectorName: feeCollectorName,
		authority:        authority,
	}
}

// SetLegacyParamSubspace sets the param subspace for migration purposes
func (k *Keeper) SetLegacyParamSubspace(ps paramtypes.Subspace) {
	k.paramSpace = ps
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// GetMinter returns the minter
func (k Keeper) GetMinter(ctx context.Context) (minter types.Minter, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := sdkCtx.KVStore(k.storeService.OpenKVStore(ctx))
	b := store.Get([]byte(types.MinterKey))
	if b == nil {
		return minter, nil
	}

	k.cdc.MustUnmarshal(b, &minter)
	return minter, nil
}

// SetMinter sets the minter
func (k Keeper) SetMinter(ctx context.Context, minter types.Minter) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := sdkCtx.KVStore(k.storeService.OpenKVStore(ctx))
	b := k.cdc.MustMarshal(&minter)
	store.Set([]byte(types.MinterKey), b)
	return nil
}

// GetParams returns the total set of minting parameters.
func (k Keeper) GetParams(ctx context.Context) (params types.Params, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if k.paramSpace.HasKeyTable() {
		k.paramSpace.GetParamSet(sdkCtx, &params)
		return params, nil
	}
	return params, nil
}

// SetParams sets the total set of minting parameters.
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	k.paramSpace.SetParamSet(sdkCtx, &params)
	return nil
}

// StakingTokenSupply implements an alias call to the underlying staking keeper's
// StakingTokenSupply to be used in BeginBlocker.
func (k Keeper) StakingTokenSupply(ctx context.Context) (math.Int, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.stakingKeeper.StakingTokenSupply(sdkCtx), nil
}

// BondedRatio implements an alias call to the underlying staking keeper's
// BondedRatio to be used in BeginBlocker.
func (k Keeper) BondedRatio(ctx context.Context) (math.LegacyDec, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.stakingKeeper.BondedRatio(sdkCtx), nil
}

// MintCoins implements an alias call to the underlying supply keeper's
// MintCoins to be used in BeginBlocker.
func (k Keeper) MintCoins(ctx context.Context, newCoins sdk.Coins) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if newCoins.Empty() {
		// skip as no coins need to be minted
		return nil
	}

	return k.bankKeeper.MintCoins(sdkCtx, types.ModuleName, newCoins)
}

// AddCollectedFees implements an alias call to the underlying supply keeper's
// SendCoinsFromModuleToModule to be used in BeginBlocker.
func (k Keeper) AddCollectedFees(ctx context.Context, fees sdk.Coins) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.bankKeeper.SendCoinsFromModuleToModule(sdkCtx, types.ModuleName, k.feeCollectorName, fees)
}
