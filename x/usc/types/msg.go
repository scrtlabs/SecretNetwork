package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Module message types.
const (
	TypeMsgMintUSC          = "mint_usc"
	TypeMsgRedeemCollateral = "redeem_collateral"
)

var (
	_ sdk.Msg = &MsgMintUSC{}
	_ sdk.Msg = &MsgRedeemCollateral{}
)

// NewMsgMintUSC builds a new MsgMintUSC sdk.Msg.
func NewMsgMintUSC(fromAddr sdk.AccAddress, coins sdk.Coins) *MsgMintUSC {
	return &MsgMintUSC{
		Address:          fromAddr.String(),
		CollateralAmount: coins,
	}
}

// Route implement the sdk.Msg interface.
func (m MsgMintUSC) Route() string {
	return RouterKey
}

// Type implement the sdk.Msg interface.
func (m MsgMintUSC) Type() string {
	return TypeMsgMintUSC
}

// GetSigners implement the sdk.Msg interface.
func (m MsgMintUSC) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Address)

	return []sdk.AccAddress{addr}
}

// ValidateBasic implement the sdk.Msg interface.
func (m MsgMintUSC) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Address); err != nil {
		return sdkErrors.ErrInvalidAddress.Wrapf("address: invalid: %v", err)
	}

	if len(m.CollateralAmount) == 0 {
		return sdkErrors.ErrInvalidCoins.Wrapf("collateral_amount: empty")
	}
	for _, coin := range m.CollateralAmount {
		if coin.Amount.LTE(sdk.ZeroInt()) {
			return sdkErrors.ErrInvalidCoins.Wrapf("collateral_amount: %s: amount must be GT 0 (%s)", coin.Denom, coin.Amount.String())
		}
		if err := sdk.ValidateDenom(coin.Denom); err != nil {
			return sdkErrors.ErrInvalidCoins.Wrapf("collateral_amount: %s: invalid denom: %v", coin.Denom, err)
		}
	}

	return nil
}

// NewMsgRedeemCollateral builds a new MsgRedeemCollateral sdk.Msg.
func NewMsgRedeemCollateral(fromAddr sdk.AccAddress, coin sdk.Coin) *MsgRedeemCollateral {
	return &MsgRedeemCollateral{
		Address:   fromAddr.String(),
		UscAmount: coin,
	}
}

// Route implement the sdk.Msg interface.
func (m MsgRedeemCollateral) Route() string {
	return RouterKey
}

// Type implement the sdk.Msg interface.
func (m MsgRedeemCollateral) Type() string {
	return TypeMsgRedeemCollateral
}

// GetSigners implement the sdk.Msg interface.
func (m MsgRedeemCollateral) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Address)

	return []sdk.AccAddress{addr}
}

// ValidateBasic implement the sdk.Msg interface.
func (m MsgRedeemCollateral) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Address); err != nil {
		return sdkErrors.ErrInvalidAddress.Wrapf("address: invalid: %v", err)
	}

	if err := sdk.ValidateDenom(m.UscAmount.Denom); err != nil {
		return sdkErrors.ErrInvalidCoins.Wrapf("usc_amount: invalid denom (%s): %v", m.UscAmount.Denom, err)
	}
	if m.UscAmount.Amount.LTE(sdk.ZeroInt()) {
		return sdkErrors.ErrInvalidCoins.Wrapf("usc_amount: amount must be GT 0 (%s)", m.UscAmount.Amount.String())
	}

	return nil
}
