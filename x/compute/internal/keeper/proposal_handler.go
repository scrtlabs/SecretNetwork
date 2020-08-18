package keeper

import (
	"fmt"

	"github.com/CosmWasm/wasmd/x/wasm/internal/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

// NewWasmProposalHandler creates a new governance Handler for wasm proposals
func NewWasmProposalHandler(k Keeper, enabledProposalTypes []types.ProposalType) govtypes.Handler {
	enabledTypes := make(map[string]struct{}, len(enabledProposalTypes))
	for i := range enabledProposalTypes {
		enabledTypes[string(enabledProposalTypes[i])] = struct{}{}
	}
	return func(ctx sdk.Context, content govtypes.Content) error {
		if content == nil {
			return sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "content must not be empty")
		}
		if _, ok := enabledTypes[content.ProposalType()]; !ok {
			return sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "unsupported wasm proposal content type: %q", content.ProposalType())
		}
		switch c := content.(type) {
		case types.StoreCodeProposal:
			return handleStoreCodeProposal(ctx, k, c)
		case types.InstantiateContractProposal:
			return handleInstantiateProposal(ctx, k, c)
		case types.MigrateContractProposal:
			return handleMigrateProposal(ctx, k, c)
		case types.UpdateAdminProposal:
			return handleUpdateAdminProposal(ctx, k, c)
		case types.ClearAdminProposal:
			return handleClearAdminProposal(ctx, k, c)
		default:
			return sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized wasm proposal content type: %T", c)
		}
	}
}

func handleStoreCodeProposal(ctx sdk.Context, k Keeper, p types.StoreCodeProposal) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	codeID, err := k.create(ctx, p.RunAs, p.WASMByteCode, p.Source, p.Builder, p.InstantiatePermission, GovAuthorizationPolicy{})
	if err != nil {
		return err
	}

	ourEvent := sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(types.AttributeKeyCodeID, fmt.Sprintf("%d", codeID)),
	)
	ctx.EventManager().EmitEvent(ourEvent)
	return nil
}

func handleInstantiateProposal(ctx sdk.Context, k Keeper, p types.InstantiateContractProposal) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	contractAddr, err := k.instantiate(ctx, p.CodeID, p.RunAs, p.Admin, p.InitMsg, p.Label, p.InitFunds, GovAuthorizationPolicy{})
	if err != nil {
		return err
	}

	ourEvent := sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(types.AttributeKeyCodeID, fmt.Sprintf("%d", p.CodeID)),
		sdk.NewAttribute(types.AttributeKeyContract, contractAddr.String()),
	)
	ctx.EventManager().EmitEvent(ourEvent)
	return nil
}

func handleMigrateProposal(ctx sdk.Context, k Keeper, p types.MigrateContractProposal) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	res, err := k.migrate(ctx, p.Contract, p.RunAs, p.CodeID, p.MigrateMsg, GovAuthorizationPolicy{})
	if err != nil {
		return err
	}

	ourEvent := sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(types.AttributeKeyContract, p.Contract.String()),
	)
	ctx.EventManager().EmitEvents(append(res.Events, ourEvent))
	return nil
}

func handleUpdateAdminProposal(ctx sdk.Context, k Keeper, p types.UpdateAdminProposal) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	if err := k.setContractAdmin(ctx, p.Contract, nil, p.NewAdmin, GovAuthorizationPolicy{}); err != nil {
		return err
	}

	ourEvent := sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(types.AttributeKeyContract, p.Contract.String()),
	)
	ctx.EventManager().EmitEvent(ourEvent)
	return nil
}

func handleClearAdminProposal(ctx sdk.Context, k Keeper, p types.ClearAdminProposal) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	if err := k.setContractAdmin(ctx, p.Contract, nil, nil, GovAuthorizationPolicy{}); err != nil {
		return err
	}
	ourEvent := sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(types.AttributeKeyContract, p.Contract.String()),
	)
	ctx.EventManager().EmitEvent(ourEvent)
	return nil
}
