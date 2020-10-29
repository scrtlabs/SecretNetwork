package types

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authexported "github.com/cosmos/cosmos-sdk/x/auth/exported"
	supplyexported "github.com/cosmos/cosmos-sdk/x/supply/exported"
)

const (
	// ModuleName is the name of the module
	ModuleName = "tokenswap"

	// StoreKey is used to register the module's store
	StoreKey = ModuleName

	// QuerierRoute is the querier route for the tokenswap module
	QuerierRoute = ModuleName
)

// TokenSwap struct containing the data of the TokenSwap. json and yaml tags are used to specify field names when marshalled
type TokenSwapRecord struct {
	BurnTxHash     EthereumTxHash  `json:"ethereum_tx_hash" yaml:"ethereum_tx_hash"`
	EthereumSender EthereumAddress `json:"ethereum_sender" yaml:"ethereum_sender"`
	Receiver       sdk.AccAddress  `json:"receiver" yaml:"receiver"`
	AmountUSCRT    sdk.Coins       `json:"amount_uscrt" yaml:"amount_uscrt"`
	Done           bool            `json:"done" yaml:"done"`
}

// TokenSwap struct containing the data of the TokenSwap. json and yaml tags are used to specify field names when marshalled
type Params struct {
	MultisigApproveAddress sdk.AccAddress `json:"minting_approver_address" yaml:"minting_approver_address"`
	MintingMultiplier      sdk.Dec        `json:"minting_multiplier" yaml:"minting_multiplier"`
	MintingEnabled         bool           `json:"minting_enabled" yaml:"minting_enabled"`
}

// NewTokenSwap Returns a new TokenSwap
func NewTokenSwapRecord(burnTxHash EthereumTxHash, ethereumSender EthereumAddress, receiver sdk.AccAddress, AmountUSCRT sdk.Coins, done bool) TokenSwapRecord {
	return TokenSwapRecord{
		BurnTxHash:     burnTxHash,
		EthereumSender: ethereumSender,
		Receiver:       receiver,
		AmountUSCRT:    AmountUSCRT,
		Done:           done,
	}
}

// String implement fmt.Stringer
func (s TokenSwapRecord) String() string {
	return strings.TrimSpace(
		fmt.Sprintf(`EthereumTxHash=%s EthereumSender=%s Receiver=%s Amount=%s`,
			s.BurnTxHash,
			s.EthereumSender,
			s.Receiver.String(),
			s.AmountUSCRT.String(),
		),
	)
}

// AccountKeeper defines the expected account keeper
type AccountKeeper interface {
	GetAccount(sdk.Context, sdk.AccAddress) authexported.Account
}

// SupplyKeeper defines the expected supply keeper
type SupplyKeeper interface {
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	MintCoins(ctx sdk.Context, name string, amt sdk.Coins) error
	BurnCoins(ctx sdk.Context, name string, amt sdk.Coins) error
	SetModuleAccount(sdk.Context, supplyexported.ModuleAccountI)
}
