package v1_0_3

import (
	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/compliance/types"
)

func MigrateStore(ctx sdk.Context, k types.ComplianceKeeper) error {
	var (
		operators       []*types.OperatorDetails
		operatorDetails *types.OperatorDetails
		issuerDetails   *types.IssuerDetails
		err             error
	)
	k.IterateOperatorDetails(ctx, func(address sdk.AccAddress) (continue_ bool) {
		operatorDetails, err = k.GetOperatorDetails(ctx, address)
		if err != nil {
			return false
		}
		if operatorDetails.OperatorType == types.OperatorType_OT_INITIAL {
			operators = append(operators, operatorDetails)
		}
		return true
	})

	if len(operators) < 1 || err != nil {
		panic(errors.Wrapf(types.ErrInvalidOperator, "empty initial operators"))
	}

	defaultIssuerCreator, _ := sdk.AccAddressFromBech32(operators[0].Operator)
	k.IterateIssuerDetails(ctx, func(address sdk.AccAddress) bool {
		issuerDetails, err = k.GetIssuerDetails(ctx, address)
		if issuerDetails == nil || err != nil {
			return false
		}
		if len(issuerDetails.Creator) < 1 {
			// In v1.0.2, only operator can create/update/remove issuer.
			// The operators who signed the transaction for creating/updating issuer were stored in
			// EventManager as event log.
			// There's no guarantee if any node in the network can fetch all the signers from EventManager during upgrade.
			// So let's initialize issuer's creator with first initial operator by default if issuer is valid
			issuerDetails.Creator = defaultIssuerCreator.String()
			_ = k.SetIssuerDetails(ctx, address, issuerDetails)
		}
		return true
	})

	return nil
}
