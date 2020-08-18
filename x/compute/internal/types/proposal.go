package types

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

type ProposalType string

const (
	ProposalTypeStoreCode           ProposalType = "StoreCode"
	ProposalTypeInstantiateContract ProposalType = "InstantiateContract"
	ProposalTypeMigrateContract     ProposalType = "MigrateContract"
	ProposalTypeUpdateAdmin         ProposalType = "UpdateAdmin"
	ProposalTypeClearAdmin          ProposalType = "ClearAdmin"
)

// DisableAllProposals contains no wasm gov types.
var DisableAllProposals []ProposalType

// EnableAllProposals contains all wasm gov types as keys.
var EnableAllProposals = []ProposalType{
	ProposalTypeStoreCode,
	ProposalTypeInstantiateContract,
	ProposalTypeMigrateContract,
	ProposalTypeUpdateAdmin,
	ProposalTypeClearAdmin,
}

// ConvertToProposals maps each key to a ProposalType and returns a typed list.
// If any string is not a valid type (in this file), then return an error
func ConvertToProposals(keys []string) ([]ProposalType, error) {
	valid := make(map[string]bool, len(EnableAllProposals))
	for _, key := range EnableAllProposals {
		valid[string(key)] = true
	}

	proposals := make([]ProposalType, len(keys))
	for i, key := range keys {
		if _, ok := valid[key]; !ok {
			return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "'%s' is not a valid ProposalType", key)
		}
		proposals[i] = ProposalType(key)
	}
	return proposals, nil
}

func init() { // register new content types with the sdk
	govtypes.RegisterProposalType(string(ProposalTypeStoreCode))
	govtypes.RegisterProposalType(string(ProposalTypeInstantiateContract))
	govtypes.RegisterProposalType(string(ProposalTypeMigrateContract))
	govtypes.RegisterProposalType(string(ProposalTypeUpdateAdmin))
	govtypes.RegisterProposalType(string(ProposalTypeClearAdmin))
	govtypes.RegisterProposalTypeCodec(StoreCodeProposal{}, "wasm/StoreCodeProposal")
	govtypes.RegisterProposalTypeCodec(InstantiateContractProposal{}, "wasm/InstantiateContractProposal")
	govtypes.RegisterProposalTypeCodec(MigrateContractProposal{}, "wasm/MigrateContractProposal")
	govtypes.RegisterProposalTypeCodec(UpdateAdminProposal{}, "wasm/UpdateAdminProposal")
	govtypes.RegisterProposalTypeCodec(ClearAdminProposal{}, "wasm/ClearAdminProposal")
}

// WasmProposal contains common proposal data.
type WasmProposal struct {
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description" yaml:"description"`
}

// GetTitle returns the title of a parameter change proposal.
func (p WasmProposal) GetTitle() string { return p.Title }

// GetDescription returns the description of a parameter change proposal.
func (p WasmProposal) GetDescription() string { return p.Description }

// ProposalRoute returns the routing key of a parameter change proposal.
func (p WasmProposal) ProposalRoute() string { return RouterKey }

// ValidateBasic validates the proposal
func (p WasmProposal) ValidateBasic() error {
	if strings.TrimSpace(p.Title) != p.Title {
		return sdkerrors.Wrap(govtypes.ErrInvalidProposalContent, "proposal title must not start/end with white spaces")
	}
	if len(p.Title) == 0 {
		return sdkerrors.Wrap(govtypes.ErrInvalidProposalContent, "proposal title cannot be blank")
	}
	if len(p.Title) > govtypes.MaxTitleLength {
		return sdkerrors.Wrapf(govtypes.ErrInvalidProposalContent, "proposal title is longer than max length of %d", govtypes.MaxTitleLength)
	}
	if strings.TrimSpace(p.Description) != p.Description {
		return sdkerrors.Wrap(govtypes.ErrInvalidProposalContent, "proposal description must not start/end with white spaces")
	}
	if len(p.Description) == 0 {
		return sdkerrors.Wrap(govtypes.ErrInvalidProposalContent, "proposal description cannot be blank")
	}
	if len(p.Description) > govtypes.MaxDescriptionLength {
		return sdkerrors.Wrapf(govtypes.ErrInvalidProposalContent, "proposal description is longer than max length of %d", govtypes.MaxDescriptionLength)
	}
	return nil
}

// StoreCodeProposal gov proposal content type to store wasm code.
type StoreCodeProposal struct {
	WasmProposal
	// RunAs is the address that "owns" the code object
	RunAs sdk.AccAddress `json:"run_as"`
	// WASMByteCode can be raw or gzip compressed
	WASMByteCode []byte `json:"wasm_byte_code"`
	// Source is a valid absolute HTTPS URI to the contract's source code, optional
	Source string `json:"source"`
	// Builder is a valid docker image name with tag, optional
	Builder string `json:"builder"`
	// InstantiatePermission to apply on contract creation, optional
	InstantiatePermission *AccessConfig `json:"instantiate_permission"`
}

// ProposalType returns the type
func (p StoreCodeProposal) ProposalType() string { return string(ProposalTypeStoreCode) }

// ValidateBasic validates the proposal
func (p StoreCodeProposal) ValidateBasic() error {
	if err := p.WasmProposal.ValidateBasic(); err != nil {
		return err
	}
	if err := sdk.VerifyAddressFormat(p.RunAs); err != nil {
		return sdkerrors.Wrap(err, "run as")
	}

	if err := validateWasmCode(p.WASMByteCode); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "code bytes %s", err.Error())
	}

	if err := validateSourceURL(p.Source); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "source %s", err.Error())
	}

	if err := validateBuilder(p.Builder); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "builder %s", err.Error())
	}
	if p.InstantiatePermission != nil {
		if err := p.InstantiatePermission.ValidateBasic(); err != nil {
			return sdkerrors.Wrap(err, "instantiate permission")
		}
	}
	return nil
}

// String implements the Stringer interface.
func (p StoreCodeProposal) String() string {
	return fmt.Sprintf(`Store Code Proposal:
  Title:       %s
  Description: %s
  Run as:      %s
  WasmCode:    %X
  Source:      %s
  Builder:     %s
`, p.Title, p.Description, p.RunAs, p.WASMByteCode, p.Source, p.Builder)
}

func (p StoreCodeProposal) MarshalYAML() (interface{}, error) {
	return struct {
		WasmProposal          `yaml:",inline"`
		RunAs                 sdk.AccAddress `yaml:"run_as"`
		WASMByteCode          string         `yaml:"wasm_byte_code"`
		Source                string         `yaml:"source"`
		Builder               string         `yaml:"builder"`
		InstantiatePermission *AccessConfig  `yaml:"instantiate_permission"`
	}{
		WasmProposal:          p.WasmProposal,
		RunAs:                 p.RunAs,
		WASMByteCode:          base64.StdEncoding.EncodeToString(p.WASMByteCode),
		Source:                p.Source,
		Builder:               p.Builder,
		InstantiatePermission: p.InstantiatePermission,
	}, nil
}

// InstantiateContractProposal gov proposal content type to instantiate a contract.
type InstantiateContractProposal struct {
	WasmProposal
	// RunAs is the address that pays the init funds
	RunAs sdk.AccAddress `json:"run_as"`
	// Admin is an optional address that can execute migrations
	Admin     sdk.AccAddress  `json:"admin,omitempty"`
	CodeID    uint64          `json:"code_id"`
	Label     string          `json:"label"`
	InitMsg   json.RawMessage `json:"init_msg"`
	InitFunds sdk.Coins       `json:"init_funds"`
}

// ProposalType returns the type
func (p InstantiateContractProposal) ProposalType() string {
	return string(ProposalTypeInstantiateContract)
}

// ValidateBasic validates the proposal
func (p InstantiateContractProposal) ValidateBasic() error {
	if err := p.WasmProposal.ValidateBasic(); err != nil {
		return err
	}
	if err := sdk.VerifyAddressFormat(p.RunAs); err != nil {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "run as")
	}

	if p.CodeID == 0 {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "code id is required")
	}

	if err := validateLabel(p.Label); err != nil {
		return err
	}

	if !p.InitFunds.IsValid() {
		return sdkerrors.ErrInvalidCoins
	}

	if len(p.Admin) != 0 {
		if err := sdk.VerifyAddressFormat(p.Admin); err != nil {
			return err
		}
	}
	return nil

}

// String implements the Stringer interface.
func (p InstantiateContractProposal) String() string {
	return fmt.Sprintf(`Instantiate Code Proposal:
  Title:       %s
  Description: %s
  Run as:      %s
  Admin:       %s
  Code id:     %d
  Label:       %s
  InitMsg:     %q
  InitFunds:   %s
`, p.Title, p.Description, p.RunAs, p.Admin, p.CodeID, p.Label, p.InitMsg, p.InitFunds)
}

func (p InstantiateContractProposal) MarshalYAML() (interface{}, error) {
	return struct {
		WasmProposal `yaml:",inline"`
		RunAs        sdk.AccAddress `yaml:"run_as"`
		Admin        sdk.AccAddress `yaml:"admin"`
		CodeID       uint64         `yaml:"code_id"`
		Label        string         `yaml:"label"`
		InitMsg      string         `yaml:"init_msg"`
		InitFunds    sdk.Coins      `yaml:"init_funds"`
	}{
		WasmProposal: p.WasmProposal,
		RunAs:        p.RunAs,
		Admin:        p.Admin,
		CodeID:       p.CodeID,
		Label:        p.Label,
		InitMsg:      string(p.InitMsg),
		InitFunds:    p.InitFunds,
	}, nil
}

// MigrateContractProposal gov proposal content type to migrate a contract.
type MigrateContractProposal struct {
	WasmProposal `yaml:",inline"`
	Contract     sdk.AccAddress  `json:"contract"`
	CodeID       uint64          `json:"code_id"`
	MigrateMsg   json.RawMessage `json:"msg"`
	// RunAs is the address that is passed to the contract's environment as sender
	RunAs sdk.AccAddress `json:"run_as"`
}

// ProposalType returns the type
func (p MigrateContractProposal) ProposalType() string { return string(ProposalTypeMigrateContract) }

// ValidateBasic validates the proposal
func (p MigrateContractProposal) ValidateBasic() error {
	if err := p.WasmProposal.ValidateBasic(); err != nil {
		return err
	}
	if p.CodeID == 0 {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "code_id is required")
	}
	if err := sdk.VerifyAddressFormat(p.Contract); err != nil {
		return sdkerrors.Wrap(err, "contract")
	}
	if err := sdk.VerifyAddressFormat(p.RunAs); err != nil {
		return sdkerrors.Wrap(err, "run as")
	}
	return nil
}

// String implements the Stringer interface.
func (p MigrateContractProposal) String() string {
	return fmt.Sprintf(`Migrate Contract Proposal:
  Title:       %s
  Description: %s
  Contract:    %s
  Code id:     %d
  Run as:      %s
  MigrateMsg   %q
`, p.Title, p.Description, p.Contract, p.CodeID, p.RunAs, p.MigrateMsg)
}

func (p MigrateContractProposal) MarshalYAML() (interface{}, error) {
	return struct {
		WasmProposal `yaml:",inline"`
		Contract     sdk.AccAddress `yaml:"contract"`
		CodeID       uint64         `yaml:"code_id"`
		MigrateMsg   string         `yaml:"msg"`
		RunAs        sdk.AccAddress `yaml:"run_as"`
	}{
		WasmProposal: p.WasmProposal,
		Contract:     p.Contract,
		CodeID:       p.CodeID,
		MigrateMsg:   string(p.MigrateMsg),
		RunAs:        p.RunAs,
	}, nil
}

// UpdateAdminProposal gov proposal content type to set an admin for a contract.
type UpdateAdminProposal struct {
	WasmProposal `yaml:",inline"`
	NewAdmin     sdk.AccAddress `json:"new_admin" yaml:"new_admin"`
	Contract     sdk.AccAddress `json:"contract" yaml:"contract"`
}

// ProposalType returns the type
func (p UpdateAdminProposal) ProposalType() string { return string(ProposalTypeUpdateAdmin) }

// ValidateBasic validates the proposal
func (p UpdateAdminProposal) ValidateBasic() error {
	if err := p.WasmProposal.ValidateBasic(); err != nil {
		return err
	}
	if err := sdk.VerifyAddressFormat(p.Contract); err != nil {
		return sdkerrors.Wrap(err, "contract")
	}
	if err := sdk.VerifyAddressFormat(p.NewAdmin); err != nil {
		return sdkerrors.Wrap(err, "new admin")
	}
	return nil
}

// String implements the Stringer interface.
func (p UpdateAdminProposal) String() string {
	return fmt.Sprintf(`Update Contract Admin Proposal:
  Title:       %s
  Description: %s
  Contract:    %s
  New Admin:   %s
`, p.Title, p.Description, p.Contract, p.NewAdmin)
}

// ClearAdminProposal gov proposal content type to clear the admin of a contract.
type ClearAdminProposal struct {
	WasmProposal `yaml:",inline"`

	Contract sdk.AccAddress `json:"contract" yaml:"contract"`
}

// ProposalType returns the type
func (p ClearAdminProposal) ProposalType() string { return string(ProposalTypeClearAdmin) }

// ValidateBasic validates the proposal
func (p ClearAdminProposal) ValidateBasic() error {
	if err := p.WasmProposal.ValidateBasic(); err != nil {
		return err
	}
	if err := sdk.VerifyAddressFormat(p.Contract); err != nil {
		return sdkerrors.Wrap(err, "contract")
	}
	return nil
}

// String implements the Stringer interface.
func (p ClearAdminProposal) String() string {
	return fmt.Sprintf(`Clear Contract Admin Proposal:
  Title:       %s
  Description: %s
  Contract:    %s
`, p.Title, p.Description, p.Contract)
}
