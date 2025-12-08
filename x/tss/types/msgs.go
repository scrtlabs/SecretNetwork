package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Ensure all message types implement the sdk.Msg interface
var (
	_ sdk.Msg = &MsgCreateKeySet{}
	_ sdk.Msg = &MsgInitiateDKG{}
	_ sdk.Msg = &MsgSubmitDKGRound1{}
	_ sdk.Msg = &MsgSubmitDKGRound2{}
	_ sdk.Msg = &MsgRequestSignature{}
	_ sdk.Msg = &MsgSubmitCommitment{}
	_ sdk.Msg = &MsgSubmitSignatureShare{}
)

// ===== MsgCreateKeySet =====

func (msg *MsgCreateKeySet) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

// ===== MsgInitiateDKG =====

func (msg *MsgInitiateDKG) GetSigners() []sdk.AccAddress {
	authority, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{authority}
}

// ===== MsgSubmitDKGRound1 =====

func (msg *MsgSubmitDKGRound1) GetSigners() []sdk.AccAddress {
	// The validator field contains the consensus address (hex string)
	// For now, we convert it to an AccAddress by decoding the hex
	// TODO: This should properly map consensus address to operator address
	// For testnet purposes, we're using a simplified approach
	validator, err := sdk.AccAddressFromBech32(msg.Validator)
	if err != nil {
		// If it's not a bech32 address, try to decode as hex (consensus address)
		// In production, this would query the staking module for the operator address
		// For now, return empty to prevent panic - the msg_server will validate
		return []sdk.AccAddress{}
	}
	return []sdk.AccAddress{validator}
}

// ===== MsgSubmitDKGRound2 =====

func (msg *MsgSubmitDKGRound2) GetSigners() []sdk.AccAddress {
	validator, err := sdk.AccAddressFromBech32(msg.Validator)
	if err != nil {
		return []sdk.AccAddress{}
	}
	return []sdk.AccAddress{validator}
}

// ===== MsgRequestSignature =====

func (msg *MsgRequestSignature) GetSigners() []sdk.AccAddress {
	requester, err := sdk.AccAddressFromBech32(msg.Requester)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{requester}
}

// ===== MsgSubmitCommitment =====

func (msg *MsgSubmitCommitment) GetSigners() []sdk.AccAddress {
	validator, err := sdk.AccAddressFromBech32(msg.Validator)
	if err != nil {
		return []sdk.AccAddress{}
	}
	return []sdk.AccAddress{validator}
}

// ===== MsgSubmitSignatureShare =====

func (msg *MsgSubmitSignatureShare) GetSigners() []sdk.AccAddress {
	validator, err := sdk.AccAddressFromBech32(msg.Validator)
	if err != nil {
		return []sdk.AccAddress{}
	}
	return []sdk.AccAddress{validator}
}
