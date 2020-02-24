package keeper

import (
	"encoding/base64"

	wasmTypes "github.com/confio/go-cosmwasm/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// ToOpaqueMsg encodes the msg using amino json encoding.
// Then it wraps it in base64 to make a string to include in OpaqueMsg
func ToOpaqueMsg(cdc *codec.Codec, msg sdk.Msg) (*wasmTypes.OpaqueMsg, error) {
	opaqueBz, err := cdc.MarshalJSON(msg)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}
	res := &wasmTypes.OpaqueMsg{
		Data: base64.StdEncoding.EncodeToString(opaqueBz),
	}
	return res, nil
}

// ParseOpaqueMsg parses msg.Data as a base64 string
// it then decodes to an sdk.Msg using amino json encoding.
func ParseOpaqueMsg(cdc *codec.Codec, msg *wasmTypes.OpaqueMsg) (sdk.Msg, error) {
	// until more is changes, format is amino json encoding, wrapped base64
	bz, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrTxDecode, err.Error())
	}
	var sdkmsg sdk.Msg
	err = cdc.UnmarshalJSON(bz, &sdkmsg)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
	}
	return sdkmsg, nil
}

func EncodeCosmosMsgContract(raw string) string {
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func DecodeCosmosMsgContract(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}
