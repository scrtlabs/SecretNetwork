package types

import (
	"fmt"
	"regexp"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// RouterKey is used to route messages and queriers to the greeter module
const RouterKey = "tokenswap"

// MsgTokenSwap defines the MsgTokenSwap Message
type MsgTokenSwap struct {
	EthereumTxHash string
	EthereumSender string
	Receiver       sdk.AccAddress
	AmountENG      sdk.Coin
}

// NewMsgTokenSwap Returns a new MsgTokenSwap
func NewMsgTokenSwap(ethereumTxHash string, ethereumSender string, receiver sdk.AccAddress, amountENG sdk.Coin) MsgTokenSwap {
	return MsgTokenSwap{
		EthereumTxHash: ethereumTxHash,
		EthereumSender: ethereumSender,
		Receiver:       receiver,
		AmountENG:      amountENG,
	}
}

// Route should return the name of the module
func (msg MsgTokenSwap) Route() string { return RouterKey }

// Type should return the action
func (msg MsgTokenSwap) Type() string { return "tokenswap" }

var ethereumTxHashRegex = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)
var ethereumAddressRegex = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

// ValidateBasic runs stateless checks on the message
func (msg MsgTokenSwap) ValidateBasic() error {
	if !ethereumTxHashRegex.MatchString(msg.EthereumTxHash) {
		return sdkerrors.Wrap(
			sdkerrors.ErrUnknownRequest,
			fmt.Sprintf(
				`Invalid EthereumTxHash %s accoding to regex '%s'`,
				msg.EthereumTxHash,
				ethereumTxHashRegex.String()
			),
		)
	}
	if !ethereumAddressRegex.MatchString(msg.EthereumSender) {
		return sdkerrors.Wrap(
			sdkerrors.ErrUnknownRequest,
			fmt.Sprintf(
				`Invalid EthereumSender %s accoding to regex '%s'`,
				msg.EthereumSender,
				ethereumAddressRegex.String()
			),
		)
	}

	if msg.Receiver.Empty() {
		return sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "Receiver cannot be empty")
	}

	if !msg.AmountENG.IsPositive() {
		return sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, fmt.Sprintf("Amont %s must be positive",msg.AmountENG.String()))
	}
	return nil
}

// GetSigners returns the addresses of those required to sign the message
func (msg MsgTokenSwap) GetSigners() []sdk.AccAddress {
	addrString := "enigma1m9he0epavsxs6f6kd829yqedldm3cdwcmwtw9y" // TODO get from genesis.json
	multisigAddress, err = sdk.AccAddressFromBech32(addrString)
	if err != nil {
		panic("cannot parse multisig address " + addrString)
	}
	return []sdk.AccAddress{multisigAddress}
}

// GetSignBytes encodes the message for signing
func (msg MsgTokenSwap) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}
