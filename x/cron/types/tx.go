package types

import (
	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgAddSchedule{}

func (msg *MsgAddSchedule) Route() string {
	return RouterKey
}

func (msg *MsgAddSchedule) Type() string {
	return "add-schedule"
}

func (msg *MsgAddSchedule) GetSigners() []sdk.AccAddress {
	authority, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil { // should never happen as valid basic rejects invalid addresses
		panic(err.Error())
	}
	return []sdk.AccAddress{authority}
}

func (msg *MsgAddSchedule) GetSignBytes() []byte {
	return ModuleCdc.MustMarshalJSON(msg)
}

func (msg *MsgAddSchedule) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return errors.Wrap(err, "authority is invalid")
	}

	if msg.Name == "" {
		return errors.Wrap(sdkerrors.ErrInvalidRequest, "name is invalid")
	}

	if msg.Period == 0 {
		return errors.Wrap(sdkerrors.ErrInvalidRequest, "period is invalid")
	}

	if len(msg.Msgs) == 0 {
		return errors.Wrap(sdkerrors.ErrInvalidRequest, "msgs should not be empty")
	}

	if _, ok := ExecutionStage_name[int32(msg.ExecutionStage)]; !ok {
		return errors.Wrap(sdkerrors.ErrInvalidRequest, "execution stage is invalid")
	}

	return nil
}

//----------------------------------------------------------------

var _ sdk.Msg = &MsgRemoveSchedule{}

func (msg *MsgRemoveSchedule) Route() string {
	return RouterKey
}

func (msg *MsgRemoveSchedule) Type() string {
	return "remove-schedule"
}

func (msg *MsgRemoveSchedule) GetSigners() []sdk.AccAddress {
	authority, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil { // should never happen as valid basic rejects invalid addresses
		panic(err.Error())
	}
	return []sdk.AccAddress{authority}
}

func (msg *MsgRemoveSchedule) GetSignBytes() []byte {
	return ModuleCdc.MustMarshalJSON(msg)
}

func (msg *MsgRemoveSchedule) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return errors.Wrap(err, "authority is invalid")
	}

	if msg.Name == "" {
		return errors.Wrap(sdkerrors.ErrInvalidRequest, "name is invalid")
	}

	return nil
}

//----------------------------------------------------------------

var _ sdk.Msg = &MsgUpdateParams{}

func (msg *MsgUpdateParams) Route() string {
	return RouterKey
}

func (msg *MsgUpdateParams) Type() string {
	return "update-params"
}

func (msg *MsgUpdateParams) GetSigners() []sdk.AccAddress {
	authority, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil { // should never happen as valid basic rejects invalid addresses
		panic(err.Error())
	}
	return []sdk.AccAddress{authority}
}

func (msg *MsgUpdateParams) GetSignBytes() []byte {
	return ModuleCdc.MustMarshalJSON(msg)
}

func (msg *MsgUpdateParams) Validate() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return errors.Wrap(err, "authority is invalid")
	}

	if _, err := sdk.AccAddressFromBech32(msg.Params.SecurityAddress); err != nil {
		return errors.Wrap(err, "security_address is invalid")
	}

	return nil
}
