package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)

// NOTE: This is required for the GetSignBytes function
func init() {
	RegisterCodec(Amino)
	Amino.Seal()
}

func RegisterCodec(cdc *codec.LegacyAmino) {}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*govv1beta1.Content)(nil),
		&VerifyIssuerProposal{},
	)
	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
