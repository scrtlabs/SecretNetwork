package keeper

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
	wasmtypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

var _ types.MsgServer = msgServer{}

type msgServer struct {
	keeper Keeper
}

func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{keeper: k}
}

func (m msgServer) StoreCode(goCtx context.Context, msg *types.MsgStoreCode) (*types.MsgStoreCodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender.String()),
		sdk.NewAttribute(types.AttributeKeySigner, msg.Sender.String()),
	))

	codeID, err := m.keeper.Create(ctx, msg.Sender, msg.WASMByteCode, msg.Source, msg.Builder)
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(types.AttributeKeyCodeID, fmt.Sprintf("%d", codeID)),
		),
	})

	return &types.MsgStoreCodeResponse{
		CodeID: codeID,
	}, nil
}

func (m msgServer) InstantiateContract(goCtx context.Context, msg *types.MsgInstantiateContract) (*types.MsgInstantiateContractResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	var adminAddr sdk.AccAddress
	var err error
	if msg.Admin != "" {
		if adminAddr, err = sdk.AccAddressFromBech32(msg.Admin); err != nil {
			return nil, errorsmod.Wrap(err, "admin")
		}
	}

	contractAddr, data, err := m.keeper.Instantiate(ctx, msg.CodeID, msg.Sender, adminAddr, msg.InitMsg, msg.Label, msg.InitFunds, msg.CallbackSig)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender.String()),
		sdk.NewAttribute(types.AttributeKeyContractAddr, contractAddr.String()),
	))

	// note: even if contractAddr == nil then contractAddr.String() is ok
	// \o/🤷🤷‍♂️🤷‍♀️🤦🤦‍♂️🤦‍♀️
	return &types.MsgInstantiateContractResponse{
		Address: contractAddr.String(),
		Data:    data,
	}, err
}

func (m msgServer) ExecuteContract(goCtx context.Context, msg *types.MsgExecuteContract) (*types.MsgExecuteContractResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender.String()),
		sdk.NewAttribute(types.AttributeKeyContractAddr, msg.Contract.String()),
	))

	data, err := m.keeper.Execute(ctx, msg.Contract, msg.Sender, msg.Msg, msg.SentFunds, msg.CallbackSig, wasmtypes.HandleTypeExecute)

	if data == nil {
		return &types.MsgExecuteContractResponse{
			Data: nil,
		}, err
	}
	return &types.MsgExecuteContractResponse{
		Data: data.Data,
	}, err
}

func (m msgServer) MigrateContract(goCtx context.Context, msg *types.MsgMigrateContract) (*types.MsgMigrateContractResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	senderAddr, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, errorsmod.Wrap(err, "sender")
	}
	contractAddr, err := sdk.AccAddressFromBech32(msg.Contract)
	if err != nil {
		return nil, errorsmod.Wrap(err, "contract")
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender),
	))

	data, err := m.keeper.Migrate(ctx, contractAddr, senderAddr, msg.CodeID, msg.Msg, msg.CallbackSig)
	if err != nil {
		return nil, err
	}

	return &types.MsgMigrateContractResponse{
		Data: data,
	}, nil
}

func (m msgServer) UpdateAdmin(goCtx context.Context, msg *types.MsgUpdateAdmin) (*types.MsgUpdateAdminResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	senderAddr, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, errorsmod.Wrap(err, "sender")
	}
	contractAddr, err := sdk.AccAddressFromBech32(msg.Contract)
	if err != nil {
		return nil, errorsmod.Wrap(err, "contract")
	}
	newAdminAddr, err := sdk.AccAddressFromBech32(msg.NewAdmin)
	if err != nil {
		return nil, errorsmod.Wrap(err, "new admin")
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender),
	))

	if err := m.keeper.UpdateContractAdmin(ctx, contractAddr, senderAddr, newAdminAddr, msg.CallbackSig); err != nil {
		return nil, err
	}

	return &types.MsgUpdateAdminResponse{}, nil
}

func (m msgServer) ClearAdmin(goCtx context.Context, msg *types.MsgClearAdmin) (*types.MsgClearAdminResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	senderAddr, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, errorsmod.Wrap(err, "sender")
	}
	contractAddr, err := sdk.AccAddressFromBech32(msg.Contract)
	if err != nil {
		return nil, errorsmod.Wrap(err, "contract")
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender),
	))

	if err := m.keeper.UpdateContractAdmin(ctx, contractAddr, senderAddr, nil, msg.CallbackSig); err != nil {
		return nil, err
	}

	return &types.MsgClearAdminResponse{}, nil
}

func (m msgServer) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if m.keeper.authority != req.Authority {
		return nil, errorsmod.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", m.keeper.authority, req.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := m.keeper.SetParams(ctx, req.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}

func (m msgServer) UpgradeProposalPassed(goCtx context.Context, msg *types.MsgUpgradeProposalPassed) (*types.MsgUpgradeProposalPassedResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeExecute,
		sdk.NewAttribute(sdk.AttributeKeySender, msg.SenderAddress),
		sdk.NewAttribute("mrenclave", string(msg.MrEnclaveHash)),
	))

	if err := api.OnUpgradeProposalPassed(msg.MrEnclaveHash); err != nil {
		return nil, err
	}

	return &types.MsgUpgradeProposalPassedResponse{}, nil
}

func (m msgServer) MigrateContractProposal(goCtx context.Context, msg *types.MsgMigrateContractProposal) (*types.MsgMigrateContractProposalResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	// Verify sender has authority (only governance module should call this)
	if m.keeper.authority != msg.Authority {
		return nil, errorsmod.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", m.keeper.authority, msg.Authority)
	}

	for _, contract := range msg.Contracts {
		contractAddr, err := sdk.AccAddressFromBech32(contract.Address)
		if err != nil {
			return nil, errorsmod.Wrap(err, "contract")
		}
		// Store the authorized migration
		m.keeper.SetAuthorizedMigration(ctx, contract.Address, contract.NewCodeId)
		err = m.keeper.UpdateContractGovernanceRequirement(ctx, contractAddr, true)
		if err != nil {
			return nil, errorsmod.Wrap(err, "updating contract governance requirement")
		}
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeMigrateContractProposal,
			sdk.NewAttribute(types.AttributeKeyContractAddr, contract.Address),
			sdk.NewAttribute(types.AttributeKeyCodeID, fmt.Sprintf("%d", contract.NewCodeId)),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Authority),
		))
	}

	return &types.MsgMigrateContractProposalResponse{}, nil
}

func (m msgServer) SetContractGovernance(goCtx context.Context, msg *types.MsgSetContractGovernance) (*types.MsgSetContractGovernanceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	_, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, errorsmod.Wrap(err, "sender")
	}

	contractAddr, err := sdk.AccAddressFromBech32(msg.ContractAddress)
	if err != nil {
		return nil, errorsmod.Wrap(err, "contract")
	}

	// Get contract info to check permissions and current state
	contractInfo := m.keeper.GetContractInfo(ctx, contractAddr)
	if contractInfo == nil {
		return nil, errorsmod.Wrap(types.ErrNotFound, "contract")
	}

	// Check if sender is contract admin
	if contractInfo.Admin != msg.Sender {
		return nil, sdkerrors.ErrUnauthorized.Wrap("only contract admin can change governance requirement")
	}

	// One-way ratchet: can only change false → true, never true → false
	if contractInfo.RequireGovernance && !msg.RequireGovernance {
		return nil, sdkerrors.ErrUnauthorized.Wrap("cannot disable governance requirement once enabled")
	}
	// Update the governance requirement
	if err := m.keeper.UpdateContractGovernanceRequirement(ctx, contractAddr, msg.RequireGovernance); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender),
		sdk.NewAttribute(types.AttributeKeyContractAddr, msg.ContractAddress),
		sdk.NewAttribute("require_governance", fmt.Sprintf("%t", msg.RequireGovernance)),
	))

	return &types.MsgSetContractGovernanceResponse{}, nil
}
