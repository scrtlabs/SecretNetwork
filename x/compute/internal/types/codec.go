package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	// "github.com/cosmos/cosmos-sdk/x/supply/exported"
)

// RegisterCodec registers the account types and interface
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgStoreCode{}, "wasm/MsgStoreCode", nil)
	cdc.RegisterConcrete(&MsgInstantiateContract{}, "wasm/MsgInstantiateContract", nil)
	cdc.RegisterConcrete(&MsgExecuteContract{}, "wasm/MsgExecuteContract", nil)
	/*
		cdc.RegisterConcrete(MsgMigrateContract{}, "wasm/MsgMigrateContract", nil)
		cdc.RegisterConcrete(MsgUpdateAdmin{}, "wasm/MsgUpdateAdmin", nil)
		cdc.RegisterConcrete(MsgClearAdmin{}, "wasm/MsgClearAdmin", nil)

		cdc.RegisterConcrete(StoreCodeProposal{}, "wasm/StoreCodeProposal", nil)
		cdc.RegisterConcrete(InstantiateContractProposal{}, "wasm/InstantiateContractProposal", nil)
		cdc.RegisterConcrete(MigrateContractProposal{}, "wasm/MigrateContractProposal", nil)
		cdc.RegisterConcrete(UpdateAdminProposal{}, "wasm/UpdateAdminProposal", nil)
		cdc.RegisterConcrete(ClearAdminProposal{}, "wasm/ClearAdminProposal", nil)
	*/
}

func RegisterInterfaces(registry types.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgStoreCode{},
		&MsgInstantiateContract{},
		&MsgExecuteContract{},
	)
}

// ModuleCdc generic sealed codec to be used throughout module
var (
	amino = codec.NewLegacyAmino()

	// ModuleCdc references the global x/wasm module codec.
	ModuleCdc = codec.NewAminoCodec(amino)
)

func init() {
	RegisterLegacyAminoCodec(amino)
	cryptocodec.RegisterCrypto(amino)
	amino.Seal()
}
