package types

import (
	"fmt"
	"strings"

	codec "github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName is the name of the module
	ModuleName = "swap"

	// StoreKey is used to register the module's store
	StoreKey = ModuleName
)

var (
	// ModuleCdc contains the types for the module that require encoding in amino
	ModuleCdc = codec.New()
)

// TokenSwap struct containing the data of the TokenSwap. json and yaml tags are used to specify field names when marshalled
type TokenSwap struct {
	EthereumTxHash string         `json:"ethereum_tx_hash" yaml:"ethereum_tx_hash"` // address of the account "sending" the greeting
	EthereumSender string         `json:"ethereum_sender" yaml:"ethereum_sender"`   // address of the account "sending" the greeting
	Receiver       sdk.AccAddress `json:"receiver" yaml:"receiver"`                 // address of the account "receiving" the greeting
	AmountENG      sdk.Coin       `json:"amount_eng" yaml:"amount_eng"`             // string body of the greeting
}

// NewTokenSwap Returns a new TokenSwap
func NewTokenSwap(ethereumTxHash string, ethereumSender string, receiver sdk.AccAddress, amountENG sdk.Coin) TokenSwap {
	return TokenSwap{
		EthereumTxHash: ethereumTxHash,
		EthereumSender: ethereumSender,
		Receiver:       receiver,
		AmountENG:      amountENG,
	}
}

// String implement fmt.Stringer
func (s TokenSwap) String() string {
	return strings.TrimSpace(
		fmt.Sprintf(`EthereumTxHash=%s EthereumSender=%s Receiver=%s AmountENG=%s`,
			s.EthereumTxHash,
			s.EthereumSender,
			s.Receiver.String(),
			s.AmountENG.String(),
		),
	)
}
