package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/SecretNetwork/x/usc/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ types.MsgServer = (*msgServer)(nil)

// msgServer implements the gRPC SDK messages service.
type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the types.MsgServer interface.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

// MintUSC implements the types.MsgServer interface.
func (k msgServer) MintUSC(goCtx context.Context, req *types.MsgMintUSC) (*types.MsgMintUSCResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	accAddr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "address parsing: %v", err)
	}

	// Convert collateral coins to USC coin
	uscCoin, colUsedCoins, err := k.ConvertCollateralsToUSC(ctx, req.CollateralAmount)
	if err != nil {
		return nil, err
	}
	if uscCoin.IsZero() {
		return nil, sdkErrors.Wrapf(sdkErrors.ErrInsufficientFunds, "can not mint USC tokens with provided collaterals")
	}

	// Transfer account's collateral coins to the module's Active pool
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, accAddr, types.ActivePoolName, colUsedCoins); err != nil {
		return nil, err
	}

	// Mint USC coin and transfer to client's account
	if err := k.bankKeeper.MintCoins(ctx, types.ActivePoolName, sdk.NewCoins(uscCoin)); err != nil {
		return nil, sdkErrors.Wrapf(types.ErrInternal, "minting USC coin (%s): %v", uscCoin, err)
	}

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ActivePoolName, accAddr, sdk.NewCoins(uscCoin)); err != nil {
		return nil, sdkErrors.Wrapf(types.ErrInternal, "sending USC coin (%s) from module to account: %v", uscCoin, err)
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		types.NewMintEvent(accAddr, uscCoin, colUsedCoins),
	)

	return &types.MsgMintUSCResponse{
		MintedAmount:      uscCoin,
		CollateralsAmount: colUsedCoins,
	}, nil
}

// RedeemCollateral implements the types.MsgServer interface.
func (k msgServer) RedeemCollateral(goCtx context.Context, req *types.MsgRedeemCollateral) (*types.MsgRedeemCollateralResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	accAddr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "address parsing: %v", err)
	}

	// Convert USC coin to collateral coins
	uscUsedCoin, colCoins, err := k.ConvertUSCToCollaterals(ctx, req.UscAmount)
	if err != nil {
		return nil, err
	}
	if colCoins.IsZero() {
		return nil, sdkErrors.Wrapf(sdkErrors.ErrInsufficientFunds, "USC amount is too small or pool funds are insufficient")
	}

	// Transfer account's USC coin to the module's Redeeming pool
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, accAddr, types.RedeemingPoolName, sdk.NewCoins(uscUsedCoin)); err != nil {
		return nil, err
	}

	// Burn USC coin
	if err := k.bankKeeper.BurnCoins(ctx, types.RedeemingPoolName, sdk.NewCoins(uscUsedCoin)); err != nil {
		return nil, sdkErrors.Wrapf(types.ErrInternal, "burning USC coin (%s): %v", req.UscAmount, err)
	}

	// Transfer collateral coins from the module's Active to Redeeming pool
	if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ActivePoolName, types.RedeemingPoolName, colCoins); err != nil {
		return nil, sdkErrors.Wrapf(types.ErrInternal, "transferring collateral coins (%s) between pools: %v", colCoins, err)
	}

	// Enqueue redeem request
	completionTime, err := k.BeginRedeeming(ctx, accAddr, colCoins)
	if err != nil {
		return nil, err
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		types.NewRedeemQueuedEvent(accAddr, uscUsedCoin, colCoins, completionTime),
	)

	return &types.MsgRedeemCollateralResponse{
		BurnedAmount:   uscUsedCoin,
		RedeemedAmount: colCoins,
		CompletionTime: completionTime,
	}, nil
}
