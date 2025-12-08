package types

import (
	"context"

	"cosmossdk.io/core/address"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AuthKeeper defines the expected interface for the Auth module.
type AuthKeeper interface {
	AddressCodec() address.Codec
	GetAccount(context.Context, sdk.AccAddress) sdk.AccountI // only used for simulation
	// Methods imported from account should be defined here
}

// BankKeeper defines the expected interface for the Bank module.
type BankKeeper interface {
	SpendableCoins(context.Context, sdk.AccAddress) sdk.Coins
	// Methods imported from bank should be defined here
}

// ParamSubspace defines the expected Subspace interface for parameters.
type ParamSubspace interface {
	Get(context.Context, []byte, interface{})
	Set(context.Context, []byte, interface{})
}

// StakingKeeper defines the expected interface for the Staking module.
type StakingKeeper interface {
	GetAllValidators(ctx context.Context) ([]ValidatorI, error)
	GetValidator(ctx context.Context, addr sdk.ValAddress) (ValidatorI, error)
}

// ValidatorI is expected interface for validators
type ValidatorI interface {
	GetOperator() string
	GetConsPubKey() (cryptotypes.PubKey, error)
	IsBonded() bool
	IsJailed() bool
}

// WasmKeeper defines the expected interface for the Wasm module.
// Used to call contracts via sudo when signatures are completed.
type WasmKeeper interface {
	Sudo(ctx context.Context, contractAddress sdk.AccAddress, msg []byte) ([]byte, error)
}
