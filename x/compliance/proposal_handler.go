package compliance

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"strconv"

	"github.com/scrtlabs/SecretNetwork/x/compliance/keeper"
	"github.com/scrtlabs/SecretNetwork/x/compliance/types"
)

func NewComplianceProposalHandler(k *keeper.Keeper) govv1beta1.Handler {
	return func(ctx sdk.Context, content govv1beta1.Content) error {
		switch c := content.(type) {
		case *types.VerifyIssuerProposal:
			return handleVerifyIssuerProposal(ctx, k, c)
		default:
			return errorsmod.Wrapf(errortypes.ErrUnknownRequest, "unrecognized %s proposal content type: %T", types.ModuleName, c)
		}
	}
}

func handleVerifyIssuerProposal(ctx sdk.Context, k *keeper.Keeper, p *types.VerifyIssuerProposal) error {
	issuer, err := sdk.AccAddressFromBech32(p.IssuerAddress)
	if err != nil {
		return err
	}

	// Issuer should exist and be verified
	exists, _ := k.IssuerExists(ctx, issuer)
	if !exists {
		return errorsmod.Wrapf(errortypes.ErrInvalidRequest, "unknown issuer address %s", p.IssuerAddress)
	}
	verified, _ := k.IsAddressVerified(ctx, issuer)
	if verified {
		return errorsmod.Wrapf(errortypes.ErrInvalidRequest, "issuer already verified %s", p.IssuerAddress)
	}

	// Set issuer verified through governance proposal
	err = k.SetAddressVerificationStatus(ctx, issuer, true)
	if err != nil {
		return err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeVerifyIssuer,
			sdk.NewAttribute(types.AttributeKeyIssuer, p.IssuerAddress),
			sdk.NewAttribute(types.AttributeKeyVerificationStatus, strconv.FormatBool(true)),
		),
	)
	return nil
}
