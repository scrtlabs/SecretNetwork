package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	regtypes "github.com/scrtlabs/SecretNetwork/x/registration"
)

// AccountKeeper defines the expected account keeper used for simulations (noalias)
type AccountKeeper interface {
	GetModuleAddress(name string) sdk.AccAddress
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	GetModuleAccount(ctx context.Context, name string) sdk.ModuleAccountI
	// Methods imported from account should be defined here
}

type RegKeeper interface {
	GetMasterKey(ctx sdk.Context, keyType string) *regtypes.MasterKey
}

// type WasmMsgServer interface {
// 	ExecuteContract(context.Context, *MsgExecuteContract) (*MsgExecuteContract, error)
// 	// Methods imported from account should be defined here
// }
