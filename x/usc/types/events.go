package types

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	EventTypeMint         = "usc_minted"
	EventTypeRedeemQueued = "collateral_redeem_queued"
	EventTypeRedeemDone   = "collateral_redeem_done"

	AttributeKeyMintedAmount   = "minted_amount"
	AttributeKeyRedeemedAmount = "redeemed_amount"
	AttributeKeyUsedAmount     = "used_amount"
	AttributeKeyCompletionTime = "completion_time"
)

// NewMintEvent creates a new USC mint event.
func NewMintEvent(accAddr sdk.AccAddress, mintedUSCCoin sdk.Coin, usedCollateralCoins sdk.Coins) sdk.Event {
	return sdk.NewEvent(
		EventTypeMint,
		sdk.NewAttribute(sdk.AttributeKeySender, accAddr.String()),
		sdk.NewAttribute(AttributeKeyMintedAmount, mintedUSCCoin.String()),
		sdk.NewAttribute(AttributeKeyUsedAmount, usedCollateralCoins.String()),
	)
}

// NewRedeemQueuedEvent creates a new redeem enqueue event.
func NewRedeemQueuedEvent(accAddr sdk.AccAddress, usedUSCCoin sdk.Coin, redeemedCollateralCoins sdk.Coins, completionTime time.Time) sdk.Event {
	return sdk.NewEvent(
		EventTypeRedeemQueued,
		sdk.NewAttribute(sdk.AttributeKeySender, accAddr.String()),
		sdk.NewAttribute(AttributeKeyUsedAmount, usedUSCCoin.String()),
		sdk.NewAttribute(AttributeKeyRedeemedAmount, redeemedCollateralCoins.String()),
		sdk.NewAttribute(AttributeKeyCompletionTime, completionTime.String()),
	)
}

// NewRedeemDoneEvent creates a new redeem dequeue event.
func NewRedeemDoneEvent(accAddr sdk.AccAddress, amount sdk.Coins, completionTime time.Time) sdk.Event {
	return sdk.NewEvent(
		EventTypeRedeemDone,
		sdk.NewAttribute(sdk.AttributeKeySender, accAddr.String()),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
		sdk.NewAttribute(AttributeKeyCompletionTime, completionTime.String()),
	)
}
