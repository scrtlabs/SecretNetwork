package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	wasmtypes "github.com/scrtlabs/SecretNetwork/x/compute"
)

// AccountKeeper defines the expected account keeper used for simulations (noalias)
type AccountKeeper interface {
	GetModuleAddress(moduleName string) sdk.AccAddress
	// Methods imported from account should be defined here
}

type WasmMsgServer interface {
	ExecuteContract(context.Context, *wasmtypes.MsgExecuteContract) (*wasmtypes.MsgExecuteContractResponse, error)
	// Methods imported from account should be defined here
}
