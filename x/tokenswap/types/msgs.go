package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// RouterKey is used to route messages and queriers to the greeter module
const RouterKey = "tokenswap"

// MsgSwapRequest defines the MsgSwapRequest Message
type MsgSwapRequest struct {
	BurnTxHash     EthereumTxHash
	EthereumSender EthereumAddress
	Receiver       sdk.AccAddress
	AmountENG      sdk.Dec
	SignerAddr     sdk.AccAddress
}

// Check in compile time that MsgSwapRequest is a sdk.Msg
var _ sdk.Msg = MsgSwapRequest{}

// NewMsgSwapRequest Returns a new MsgSwapRequest
func NewMsgSwapRequest(burnTxHash EthereumTxHash, ethereumSender EthereumAddress, receiver sdk.AccAddress, signerAddr sdk.AccAddress, amountENG sdk.Dec) MsgSwapRequest {
	return MsgSwapRequest{
		BurnTxHash:     burnTxHash,
		EthereumSender: ethereumSender,
		Receiver:       receiver,
		AmountENG:      amountENG,
		SignerAddr:     signerAddr,
	}
}

// Route should return the name of the module
func (msg MsgSwapRequest) Route() string { return RouterKey }

// Type should return the action
func (msg MsgSwapRequest) Type() string { return "tokenswap" }

// ValidateBasic runs stateless checks on the message
func (msg MsgSwapRequest) ValidateBasic() error {
	err := msg.ValidateAmount()
	if err != nil {
		return err
	}

	err = msg.validateEthSender()
	if err != nil {
		return err
	}

	err = msg.ValidateTxHash()
	if err != nil {
		return err
	}

	err = msg.ValidateReceiver()
	if err != nil {
		return err
	}
	return nil
}

func (msg MsgSwapRequest) ValidateAmount() error {
	if msg.AmountENG.IsZero() {
		return sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "amount to swap must be positive")
	}
	if !msg.AmountENG.Equal(sdk.NewDecFromInt(msg.AmountENG.RoundInt())) {
		return sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "amount to swap must be an integer")
	}
	if msg.AmountENG.LT(sdk.NewDec(100)) {
		return sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "amount cannot be under 100, due to lost precision from ENG dust <-> uSCRT")
	}
	return nil
}

func (msg MsgSwapRequest) ValidateReceiver() error {
	if msg.Receiver.Empty() {
		return sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "Receiver cannot be empty")
	}
	return nil
}

func (msg MsgSwapRequest) ValidateTxHash() error {
	if msg.BurnTxHash.Empty() {
		return sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "Receiver cannot be empty")
	}
	return nil
}

func (msg MsgSwapRequest) validateEthSender() error {
	if msg.EthereumSender.Empty() {
		return sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "Receiver cannot be empty")
	}
	return nil
}

// GetSigners returns the addresses of those required to sign the message
func (msg MsgSwapRequest) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.SignerAddr}
}

// GetSignBytes encodes the message for signing
func (msg MsgSwapRequest) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}
