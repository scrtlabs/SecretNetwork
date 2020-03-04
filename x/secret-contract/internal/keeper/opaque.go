package keeper

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	wasmTypes "github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/types"
)

// ToCosmosMsg encodes an sdk msg using amino json encoding.
// Then wraps it as an opaque message
func ToCosmosMsg(cdc *codec.Codec, msg sdk.Msg) (wasmTypes.CosmosMsg, error) {
	opaqueBz, err := cdc.MarshalJSON(msg)
	if err != nil {
		return wasmTypes.CosmosMsg{}, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}
	res := wasmTypes.CosmosMsg{
		Opaque: &wasmTypes.OpaqueMsg{
			Data: opaqueBz,
		},
	}
	return res, nil
}

// ParseOpaqueMsg decodes msg.Data to an sdk.Msg using amino json encoding.
func ParseOpaqueMsg(cdc *codec.Codec, msg *wasmTypes.OpaqueMsg) (sdk.Msg, error) {
	// until more is changes, format is amino json encoding, wrapped base64
	var sdkmsg sdk.Msg
	err := cdc.UnmarshalJSON(msg.Data, &sdkmsg)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
	}
	return sdkmsg, nil
}
