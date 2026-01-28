package types

import (
	"encoding/hex"
	fmt "fmt"
	"strings"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (msg MsgStoreCode) Route() string {
	return RouterKey
}

func (msg MsgStoreCode) Type() string {
	return "store-code"
}

func (msg MsgStoreCode) ValidateBasic() error {
	if err := sdk.VerifyAddressFormat([]byte(msg.Sender)); err != nil {
		return err
	}

	if err := validateWasmCode(msg.WASMByteCode); err != nil {
		return sdkerrors.ErrInvalidRequest.Wrapf("code bytes %s", err.Error())
	}

	if err := validateSourceURL(msg.Source); err != nil {
		return sdkerrors.ErrInvalidRequest.Wrapf("source %s", err.Error())
	}

	if err := validateBuilder(msg.Builder); err != nil {
		return sdkerrors.ErrInvalidRequest.Wrapf("builder %s", err.Error())
	}

	return nil
}

func (msg MsgStoreCode) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (msg MsgStoreCode) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{[]byte(msg.Sender)}
}

func (msg MsgInstantiateContract) Route() string {
	return RouterKey
}

func (msg MsgInstantiateContract) Type() string {
	return "instantiate"
}

func (msg MsgInstantiateContract) ValidateBasic() error {
	if err := sdk.VerifyAddressFormat([]byte(msg.Sender)); err != nil {
		return err
	}

	if msg.CodeID == 0 {
		return sdkerrors.ErrInvalidRequest.Wrap("code_id is required")
	}

	if err := validateLabel(msg.Label); err != nil {
		return err
	}

	if !msg.InitFunds.IsValid() {
		return sdkerrors.ErrInvalidCoins
	}

	return nil
}

func (msg MsgInstantiateContract) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (msg MsgInstantiateContract) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{[]byte(msg.Sender)}
}

func (msg MsgExecuteContract) Route() string {
	return RouterKey
}

func (msg MsgExecuteContract) Type() string {
	return "execute"
}

func (msg MsgExecuteContract) ValidateBasic() error {
	if err := sdk.VerifyAddressFormat(msg.Sender); err != nil {
		return err
	}
	if err := sdk.VerifyAddressFormat(msg.Contract); err != nil {
		return err
	}

	if !msg.SentFunds.IsValid() {
		return sdkerrors.ErrInvalidCoins.Wrap("sentFunds")
	}

	return nil
}

func (msg MsgExecuteContract) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (msg MsgExecuteContract) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Sender}
}

func (msg MsgMigrateContract) Route() string {
	return RouterKey
}

func (msg MsgMigrateContract) Type() string {
	return "migrate"
}

func (msg MsgMigrateContract) ValidateBasic() error {
	if msg.CodeID == 0 {
		return sdkerrors.ErrInvalidRequest.Wrap("code id is required")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Sender); err != nil {
		return errorsmod.Wrap(err, "sender")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Contract); err != nil {
		return errorsmod.Wrap(err, "contract")
	}

	return nil
}

func (msg MsgMigrateContract) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (msg MsgMigrateContract) GetSigners() []sdk.AccAddress {
	senderAddr, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil { // should never happen as valid basic rejects invalid addresses
		panic(err.Error())
	}
	return []sdk.AccAddress{senderAddr}
}

// GetFunds returns tokens send to the contract
func (msg MsgMigrateContract) GetFunds() sdk.Coins {
	return sdk.NewCoins()
}

func (msg MsgUpdateAdmin) Route() string {
	return RouterKey
}

func (msg MsgUpdateAdmin) Type() string {
	return "update-contract-admin"
}

func (msg MsgUpdateAdmin) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Sender); err != nil {
		return errorsmod.Wrap(err, "sender")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Contract); err != nil {
		return errorsmod.Wrap(err, "contract")
	}
	if _, err := sdk.AccAddressFromBech32(msg.NewAdmin); err != nil {
		return errorsmod.Wrap(err, "new admin")
	}
	if strings.EqualFold(msg.Sender, msg.NewAdmin) {
		return errorsmod.Wrap(ErrInvalidMsg, "new admin is the same as the old")
	}
	return nil
}

func (msg MsgUpdateAdmin) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (msg MsgUpdateAdmin) GetSigners() []sdk.AccAddress {
	senderAddr, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil { // should never happen as valid basic rejects invalid addresses
		panic(err.Error())
	}
	return []sdk.AccAddress{senderAddr}
}

func (msg MsgClearAdmin) Route() string {
	return RouterKey
}

func (msg MsgClearAdmin) Type() string {
	return "clear-contract-admin"
}

func (msg MsgClearAdmin) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Sender); err != nil {
		return errorsmod.Wrap(err, "sender")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Contract); err != nil {
		return errorsmod.Wrap(err, "contract")
	}
	return nil
}

func (msg MsgClearAdmin) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (msg MsgClearAdmin) GetSigners() []sdk.AccAddress {
	senderAddr, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil { // should never happen as valid basic rejects invalid addresses
		panic(err.Error())
	}
	return []sdk.AccAddress{senderAddr}
}

func (msg MsgUpgradeProposalPassed) Route() string {
	return RouterKey
}

func (msg MsgUpgradeProposalPassed) Type() string {
	return "upgrade-proposal-passed"
}

func (msg MsgUpgradeProposalPassed) ValidateBasic() error {
	if err := sdk.VerifyAddressFormat([]byte(msg.SenderAddress)); err != nil {
		return err
	}
	if len(msg.MrEnclaveHash) != 32 {
		return sdkerrors.ErrInvalidRequest.Wrap("MREnclave hash length is not equal 32!")
	}
	return nil
}

func (msg MsgUpgradeProposalPassed) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (msg MsgUpgradeProposalPassed) GetSigners() []sdk.AccAddress {
	senderAddr, err := sdk.AccAddressFromBech32(msg.SenderAddress)
	if err != nil { // should never happen as valid basic rejects invalid addresses
		panic(err.Error())
	}
	return []sdk.AccAddress{senderAddr}
}

func (msg MsgContractGovernanceProposal) Route() string {
	return RouterKey
}

func (msg MsgContractGovernanceProposal) Type() string {
	return "migrate-contract-proposal"
}

func (msg MsgContractGovernanceProposal) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return errorsmod.Wrap(err, "authority")
	}
	for _, contract := range msg.Contracts {
		if _, err := sdk.AccAddressFromBech32(contract.Address); err != nil {
			return errorsmod.Wrap(err, "contract")
		}
	}
	for _, adminUpdate := range msg.AdminUpdates {
		if _, err := sdk.AccAddressFromBech32(adminUpdate.Address); err != nil {
			return errorsmod.Wrap(err, "contract")
		}
		if adminUpdate.NewAdmin == "" {
			continue
		}
		if _, err := sdk.AccAddressFromBech32(adminUpdate.NewAdmin); err != nil {
			return errorsmod.Wrap(err, "new admin")
		}
	}
	return nil
}

func (msg MsgContractGovernanceProposal) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (msg MsgContractGovernanceProposal) GetSigners() []sdk.AccAddress {
	senderAddr, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil { // should never happen as valid basic rejects invalid addresses
		panic(err.Error())
	}
	return []sdk.AccAddress{senderAddr}
}

func (msg MsgSetContractGovernance) Route() string {
	return RouterKey
}

func (msg MsgSetContractGovernance) Type() string {
	return "set-contract-governance"
}

func (msg MsgSetContractGovernance) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Sender); err != nil {
		return errorsmod.Wrap(err, "sender")
	}
	if _, err := sdk.AccAddressFromBech32(msg.ContractAddress); err != nil {
		return errorsmod.Wrap(err, "contract")
	}
	return nil
}

func (msg MsgSetContractGovernance) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (msg MsgSetContractGovernance) GetSigners() []sdk.AccAddress {
	senderAddr, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil { // should never happen as valid basic rejects invalid addresses
		panic(err.Error())
	}
	return []sdk.AccAddress{senderAddr}
}

func (msg MsgUpdateMachineWhitelistProposal) Route() string {
	return RouterKey
}

func (msg MsgUpdateMachineWhitelistProposal) Type() string {
	return "update-machine-whitelist-proposal"
}

func ParseHexList(s string) ([][]byte, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil // or empty slice, your choice
	}

	parts := strings.Split(s, ",")
	out := make([][]byte, 0, len(parts))

	for i, p := range parts {
		p = strings.TrimSpace(p)

		b, err := hex.DecodeString(p)
		if err != nil {
			return nil, fmt.Errorf("invalid hex token #%d (%q): %w", i, p, err)
		}

		out = append(out, b)
	}

	return out, nil
}

func (msg MsgUpdateMachineWhitelistProposal) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return errorsmod.Wrap(err, "invalid authority")
	}

	ids, err := ParseHexList(msg.MachineId)
	if err != nil {
		return errorsmod.Wrap(ErrInvalid, "machine_id malformed")
	}
	if len(ids) == 0 {
		return errorsmod.Wrap(ErrInvalid, "machine_id must not be empty")
	}

	return nil
}

func (msg MsgUpdateMachineWhitelistProposal) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (msg MsgUpdateMachineWhitelistProposal) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic(err.Error())
	}
	return []sdk.AccAddress{addr}
}

func (msg MsgUpdateMachineWhitelist) Route() string {
	return RouterKey
}

func (msg MsgUpdateMachineWhitelist) Type() string {
	return "update-machine-whitelist"
}

func (msg MsgUpdateMachineWhitelist) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Sender); err != nil {
		return errorsmod.Wrap(err, "invalid sender address")
	}

	if msg.ProposalId == 0 {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "proposal ID cannot be zero")
	}

	ids, err := ParseHexList(msg.MachineId)
	if err != nil {
		return errorsmod.Wrap(ErrInvalid, "machine_id malformed")
	}
	if len(ids) == 0 {
		return errorsmod.Wrap(ErrInvalid, "machine_id must not be empty")
	}

	return nil
}

func (msg MsgUpdateMachineWhitelist) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

func (msg MsgUpdateMachineWhitelist) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Sender)
	return []sdk.AccAddress{addr}
}
