package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	govcdc "github.com/cosmos/cosmos-sdk/x/gov/codec"
	"github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

const (
	ProposalTypeVerifyIssuer string = "VerifyIssuer"
)

// Implements Proposal Interface
var (
	_ v1beta1.Content = &VerifyIssuerProposal{}
)

func init() {
	v1beta1.RegisterProposalType(ProposalTypeVerifyIssuer)
	govcdc.ModuleCdc.Amino.RegisterConcrete(&VerifyIssuerProposal{}, "compliance/VerifyIssuerProposal", nil)
}

// NewVerifyIssuerProposal returns new instance of VerifyIssuerProposal
func NewVerifyIssuerProposal(title, description string, issuerAddress string) v1beta1.Content {
	return &VerifyIssuerProposal{
		Title:         title,
		Description:   description,
		IssuerAddress: issuerAddress,
	}
}

// ProposalRoute returns router key for this proposal
func (*VerifyIssuerProposal) ProposalRoute() string {
	return RouterKey
}

// ProposalType returns proposal type for this proposal
func (*VerifyIssuerProposal) ProposalType() string {
	return ProposalTypeVerifyIssuer
}

// ValidateBasic performs a stateless check of proposal fields
func (v *VerifyIssuerProposal) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(v.IssuerAddress)
	if err != nil {
		return err
	}
	return v1beta1.ValidateAbstract(v)
}
