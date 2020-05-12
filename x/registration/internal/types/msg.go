package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	ra "github.com/enigmampc/EnigmaBlockchain/x/registration/remote_attestation"
)

const (
	MaxCertificateSize = 20 * 1024
)

type PublicKey []byte

//type SetMasterKey struct {
//	Sender sdk.AccAddress `json:"sender" yaml:"sender"`
//	// Certificate can be raw or gzip compressed
//	MasterPublicKey PublicKey `json:"pk_io" yaml:"pk_io"`
//	// Node ID is the identifier of the node we're going to fun
//	// PubKey NodeID `json:"node_id" yaml:"node_id"`
//}
//
//func (msg SetMasterKey) Route() string {
//	return RouterKey
//}
//
//func (msg SetMasterKey) Type() string {
//	return "master-public"
//}
//
//func (msg SetMasterKey) ValidateBasic() error {
//	if err := sdk.VerifyAddressFormat(msg.Sender); err != nil {
//		return err
//	}
//
//	if len(msg.MasterPublicKey) == 64 {
//		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "Broadcasted master key cannot be empty")
//	}
//
//	return nil
//}

type RaAuthenticate struct {
	Sender sdk.AccAddress `json:"sender" yaml:"sender"`
	// Certificate can be raw or gzip compressed
	Certificate ra.Certificate `json:"ra_cert" yaml:"ra_cert"`
	// Node ID is the identifier of the node we're going to fun
	// PubKey NodeID `json:"node_id" yaml:"node_id"`
}

func (msg RaAuthenticate) Route() string {
	return RouterKey
}

func (msg RaAuthenticate) Type() string {
	return "node-auth"
}

func (msg RaAuthenticate) ValidateBasic() error {
	if err := sdk.VerifyAddressFormat(msg.Sender); err != nil {
		return err
	}

	if len(msg.Certificate) == 0 {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "Authenticating certificate cannot be empty")
	}

	if len(msg.Certificate) > MaxCertificateSize {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "certificate length too large")
	}

	//if msg.PubKey == nil {
	//	return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "Node ID cannot be empty")
	//}

	return validateCertificate(msg.Certificate)
}

func (msg RaAuthenticate) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

func (msg RaAuthenticate) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Sender}
}

func validateCertificate(cert ra.Certificate) error {
	// todo: add public key verification
	_, err := ra.VerifyRaCert(cert)
	if err != nil {
		panic(err)
	}

	return nil
}
