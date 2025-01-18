package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	ra "github.com/scrtlabs/SecretNetwork/x/registration/remote_attestation"
	"golang.org/x/xerrors"
)

type PublicKey []byte

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
		return sdkerrors.ErrInvalidRequest.Wrap("Authenticating certificate cannot be empty")
	}

	return validateCertificate(msg.Certificate)
}

func (msg RaAuthenticate) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (msg RaAuthenticate) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Sender}
}

func validateCertificate(cert ra.Certificate) error {
	// todo: add public key verification
	_, err := ra.VerifyCombinedCert(cert)
	if err != nil {
		return xerrors.Errorf("Certificate validation failed: %v", err)
	}

	return nil
}
