package types

import (
	"bytes"
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestValidateWasmProposal(t *testing.T) {
	specs := map[string]struct {
		src    WasmProposal
		expErr bool
	}{
		"all good": {src: WasmProposal{
			Title:       "Foo",
			Description: "Bar",
		}},
		"prevent empty title": {
			src: WasmProposal{
				Description: "Bar",
			},
			expErr: true,
		},
		"prevent white space only title": {
			src: WasmProposal{
				Title:       " ",
				Description: "Bar",
			},
			expErr: true,
		},
		"prevent leading white spaces in title": {
			src: WasmProposal{
				Title:       " Foo",
				Description: "Bar",
			},
			expErr: true,
		},
		"prevent title exceeds max length ": {
			src: WasmProposal{
				Title:       strings.Repeat("a", govtypes.MaxTitleLength+1),
				Description: "Bar",
			},
			expErr: true,
		},
		"prevent empty description": {
			src: WasmProposal{
				Title: "Foo",
			},
			expErr: true,
		},
		"prevent leading white spaces in description": {
			src: WasmProposal{
				Title:       "Foo",
				Description: " Bar",
			},
			expErr: true,
		},
		"prevent white space only description": {
			src: WasmProposal{
				Title:       "Foo",
				Description: " ",
			},
			expErr: true,
		},
		"prevent descr exceeds max length ": {
			src: WasmProposal{
				Title:       "Foo",
				Description: strings.Repeat("a", govtypes.MaxDescriptionLength+1),
			},
			expErr: true,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			err := spec.src.ValidateBasic()
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateStoreCodeProposal(t *testing.T) {
	var (
		anyAddress     sdk.AccAddress = bytes.Repeat([]byte{0x0}, sdk.AddrLen)
		invalidAddress sdk.AccAddress = bytes.Repeat([]byte{0x1}, sdk.AddrLen-1)
	)

	specs := map[string]struct {
		src    StoreCodeProposal
		expErr bool
	}{
		"all good": {
			src: StoreCodeProposalFixture(),
		},
		"with instantiate permission": {
			src: StoreCodeProposalFixture(func(p *StoreCodeProposal) {
				accessConfig := OnlyAddress.With(anyAddress)
				p.InstantiatePermission = &accessConfig
			}),
		},

		"without source": {
			src: StoreCodeProposalFixture(func(p *StoreCodeProposal) {
				p.Source = ""
			}),
		},
		"base data missing": {
			src: StoreCodeProposalFixture(func(p *StoreCodeProposal) {
				p.WasmProposal = WasmProposal{}
			}),
			expErr: true,
		},
		"run_as missing": {
			src: StoreCodeProposalFixture(func(p *StoreCodeProposal) {
				p.RunAs = nil
			}),
			expErr: true,
		},
		"run_as invalid": {
			src: StoreCodeProposalFixture(func(p *StoreCodeProposal) {
				p.RunAs = invalidAddress
			}),
			expErr: true,
		},
		"wasm code missing": {
			src: StoreCodeProposalFixture(func(p *StoreCodeProposal) {
				p.WASMByteCode = nil
			}),
			expErr: true,
		},
		"wasm code invalid": {
			src: StoreCodeProposalFixture(func(p *StoreCodeProposal) {
				p.WASMByteCode = bytes.Repeat([]byte{0x0}, MaxWasmSize+1)
			}),
			expErr: true,
		},
		"source invalid": {
			src: StoreCodeProposalFixture(func(p *StoreCodeProposal) {
				p.Source = "not an url"
			}),
			expErr: true,
		},
		"builder invalid": {
			src: StoreCodeProposalFixture(func(p *StoreCodeProposal) {
				p.Builder = "not a builder"
			}),
			expErr: true,
		},
		"with invalid instantiate permission": {
			src: StoreCodeProposalFixture(func(p *StoreCodeProposal) {
				p.InstantiatePermission = &AccessConfig{}
			}),
			expErr: true,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			err := spec.src.ValidateBasic()
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateInstantiateContractProposal(t *testing.T) {
	var (
		invalidAddress sdk.AccAddress = bytes.Repeat([]byte{0x1}, sdk.AddrLen-1)
	)

	specs := map[string]struct {
		src    InstantiateContractProposal
		expErr bool
	}{
		"all good": {
			src: InstantiateContractProposalFixture(),
		},
		"without admin": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.Admin = nil
			}),
		},
		"without init msg": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.InitMsg = nil
			}),
		},
		"without init funds": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.InitFunds = nil
			}),
		},
		"base data missing": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.WasmProposal = WasmProposal{}
			}),
			expErr: true,
		},
		"run_as missing": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.RunAs = nil
			}),
			expErr: true,
		},
		"run_as invalid": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.RunAs = invalidAddress
			}),
			expErr: true,
		},
		"admin invalid": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.Admin = invalidAddress
			}),
			expErr: true,
		},
		"code id empty": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.CodeID = 0
			}),
			expErr: true,
		},
		"label empty": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.Label = ""
			}),
			expErr: true,
		},
		"init funds negative": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.InitFunds = sdk.Coins{{Denom: "foo", Amount: sdk.NewInt(-1)}}
			}),
			expErr: true,
		},
		"init funds with duplicates": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.InitFunds = sdk.Coins{{Denom: "foo", Amount: sdk.NewInt(1)}, {Denom: "foo", Amount: sdk.NewInt(2)}}
			}),
			expErr: true,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			err := spec.src.ValidateBasic()
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMigrateContractProposal(t *testing.T) {
	var (
		invalidAddress sdk.AccAddress = bytes.Repeat([]byte{0x1}, sdk.AddrLen-1)
	)

	specs := map[string]struct {
		src    MigrateContractProposal
		expErr bool
	}{
		"all good": {
			src: MigrateContractProposalFixture(),
		},
		"without migrate msg": {
			src: MigrateContractProposalFixture(func(p *MigrateContractProposal) {
				p.MigrateMsg = nil
			}),
		},
		"base data missing": {
			src: MigrateContractProposalFixture(func(p *MigrateContractProposal) {
				p.WasmProposal = WasmProposal{}
			}),
			expErr: true,
		},
		"contract missing": {
			src: MigrateContractProposalFixture(func(p *MigrateContractProposal) {
				p.Contract = nil
			}),
			expErr: true,
		},
		"contract invalid": {
			src: MigrateContractProposalFixture(func(p *MigrateContractProposal) {
				p.Contract = invalidAddress
			}),
			expErr: true,
		},
		"code id empty": {
			src: MigrateContractProposalFixture(func(p *MigrateContractProposal) {
				p.CodeID = 0
			}),
			expErr: true,
		},
		"run_as missing": {
			src: MigrateContractProposalFixture(func(p *MigrateContractProposal) {
				p.RunAs = nil
			}),
			expErr: true,
		},
		"run_as invalid": {
			src: MigrateContractProposalFixture(func(p *MigrateContractProposal) {
				p.RunAs = invalidAddress
			}),
			expErr: true,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			err := spec.src.ValidateBasic()
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateUpdateAdminProposal(t *testing.T) {
	var (
		invalidAddress sdk.AccAddress = bytes.Repeat([]byte{0x1}, sdk.AddrLen-1)
	)

	specs := map[string]struct {
		src    UpdateAdminProposal
		expErr bool
	}{
		"all good": {
			src: UpdateAdminProposalFixture(),
		},
		"base data missing": {
			src: UpdateAdminProposalFixture(func(p *UpdateAdminProposal) {
				p.WasmProposal = WasmProposal{}
			}),
			expErr: true,
		},
		"contract missing": {
			src: UpdateAdminProposalFixture(func(p *UpdateAdminProposal) {
				p.Contract = nil
			}),
			expErr: true,
		},
		"contract invalid": {
			src: UpdateAdminProposalFixture(func(p *UpdateAdminProposal) {
				p.Contract = invalidAddress
			}),
			expErr: true,
		},
		"admin missing": {
			src: UpdateAdminProposalFixture(func(p *UpdateAdminProposal) {
				p.NewAdmin = nil
			}),
			expErr: true,
		},
		"admin invalid": {
			src: UpdateAdminProposalFixture(func(p *UpdateAdminProposal) {
				p.NewAdmin = invalidAddress
			}),
			expErr: true,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			err := spec.src.ValidateBasic()
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateClearAdminProposal(t *testing.T) {
	var (
		invalidAddress sdk.AccAddress = bytes.Repeat([]byte{0x1}, sdk.AddrLen-1)
	)

	specs := map[string]struct {
		src    ClearAdminProposal
		expErr bool
	}{
		"all good": {
			src: ClearAdminProposalFixture(),
		},
		"base data missing": {
			src: ClearAdminProposalFixture(func(p *ClearAdminProposal) {
				p.WasmProposal = WasmProposal{}
			}),
			expErr: true,
		},
		"contract missing": {
			src: ClearAdminProposalFixture(func(p *ClearAdminProposal) {
				p.Contract = nil
			}),
			expErr: true,
		},
		"contract invalid": {
			src: ClearAdminProposalFixture(func(p *ClearAdminProposal) {
				p.Contract = invalidAddress
			}),
			expErr: true,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			err := spec.src.ValidateBasic()
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProposalStrings(t *testing.T) {
	specs := map[string]struct {
		src gov.Content
		exp string
	}{
		"store code": {
			src: StoreCodeProposalFixture(func(p *StoreCodeProposal) {
				p.WASMByteCode = []byte{01, 02, 03, 04, 05, 06, 07, 0x08, 0x09, 0x0a}
			}),
			exp: `Store Code Proposal:
  Title:       Foo
  Description: Bar
  Run as:      cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
  WasmCode:    0102030405060708090A
  Source:      https://example.com/code
  Builder:     foo/bar:latest
`,
		},
		"instantiate contract": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.InitFunds = sdk.Coins{{Denom: "foo", Amount: sdk.NewInt(1)}, {Denom: "bar", Amount: sdk.NewInt(2)}}
			}),
			exp: `Instantiate Code Proposal:
  Title:       Foo
  Description: Bar
  Run as:      cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
  Admin:       cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
  Code id:     1
  Label:       testing
  InitMsg:     "{\"verifier\":\"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du\",\"beneficiary\":\"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du\"}"
  InitFunds:   1foo,2bar
`,
		},
		"instantiate contract without funds": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) { p.InitFunds = nil }),
			exp: `Instantiate Code Proposal:
  Title:       Foo
  Description: Bar
  Run as:      cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
  Admin:       cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
  Code id:     1
  Label:       testing
  InitMsg:     "{\"verifier\":\"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du\",\"beneficiary\":\"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du\"}"
  InitFunds:   
`,
		},
		"instantiate contract without admin": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) { p.Admin = nil }),
			exp: `Instantiate Code Proposal:
  Title:       Foo
  Description: Bar
  Run as:      cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
  Admin:       
  Code id:     1
  Label:       testing
  InitMsg:     "{\"verifier\":\"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du\",\"beneficiary\":\"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du\"}"
  InitFunds:   
`,
		},
		"migrate contract": {
			src: MigrateContractProposalFixture(),
			exp: `Migrate Contract Proposal:
  Title:       Foo
  Description: Bar
  Contract:    cosmos18vd8fpwxzck93qlwghaj6arh4p7c5n89uzcee5
  Code id:     1
  Run as:      cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
  MigrateMsg   "{\"verifier\":\"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du\"}"
`,
		},
		"update admin": {
			src: UpdateAdminProposalFixture(),
			exp: `Update Contract Admin Proposal:
  Title:       Foo
  Description: Bar
  Contract:    cosmos18vd8fpwxzck93qlwghaj6arh4p7c5n89uzcee5
  New Admin:   cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
`,
		},
		"clear admin": {
			src: ClearAdminProposalFixture(),
			exp: `Clear Contract Admin Proposal:
  Title:       Foo
  Description: Bar
  Contract:    cosmos18vd8fpwxzck93qlwghaj6arh4p7c5n89uzcee5
`,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			assert.Equal(t, spec.exp, spec.src.String())
		})
	}
}

func TestProposalYaml(t *testing.T) {
	specs := map[string]struct {
		src gov.Content
		exp string
	}{
		"store code": {
			src: StoreCodeProposalFixture(func(p *StoreCodeProposal) {
				p.WASMByteCode = []byte{01, 02, 03, 04, 05, 06, 07, 0x08, 0x09, 0x0a}
			}),
			exp: `title: Foo
description: Bar
run_as: cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
wasm_byte_code: AQIDBAUGBwgJCg==
source: https://example.com/code
builder: foo/bar:latest
instantiate_permission: null
`,
		},
		"instantiate contract": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) {
				p.InitFunds = sdk.Coins{{Denom: "foo", Amount: sdk.NewInt(1)}, {Denom: "bar", Amount: sdk.NewInt(2)}}
			}),
			exp: `title: Foo
description: Bar
run_as: cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
admin: cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
code_id: 1
label: testing
init_msg: '{"verifier":"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du","beneficiary":"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du"}'
init_funds:
- denom: foo
  amount: "1"
- denom: bar
  amount: "2"
`,
		},
		"instantiate contract without funds": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) { p.InitFunds = nil }),
			exp: `title: Foo
description: Bar
run_as: cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
admin: cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
code_id: 1
label: testing
init_msg: '{"verifier":"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du","beneficiary":"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du"}'
init_funds: []
`,
		},
		"instantiate contract without admin": {
			src: InstantiateContractProposalFixture(func(p *InstantiateContractProposal) { p.Admin = nil }),
			exp: `title: Foo
description: Bar
run_as: cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
admin: ""
code_id: 1
label: testing
init_msg: '{"verifier":"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du","beneficiary":"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du"}'
init_funds: []
`,
		},
		"migrate contract": {
			src: MigrateContractProposalFixture(),
			exp: `title: Foo
description: Bar
contract: cosmos18vd8fpwxzck93qlwghaj6arh4p7c5n89uzcee5
code_id: 1
msg: '{"verifier":"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du"}'
run_as: cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
`,
		},
		"update admin": {
			src: UpdateAdminProposalFixture(),
			exp: `title: Foo
description: Bar
new_admin: cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du
contract: cosmos18vd8fpwxzck93qlwghaj6arh4p7c5n89uzcee5
`,
		},
		"clear admin": {
			src: ClearAdminProposalFixture(),
			exp: `title: Foo
description: Bar
contract: cosmos18vd8fpwxzck93qlwghaj6arh4p7c5n89uzcee5
`,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			v, err := yaml.Marshal(&spec.src)
			require.NoError(t, err)
			assert.Equal(t, spec.exp, string(v))
		})
	}
}

func TestConvertToProposals(t *testing.T) {
	cases := map[string]struct {
		input     string
		isError   bool
		proposals []ProposalType
	}{
		"one proper item": {
			input:     "UpdateAdmin",
			proposals: []ProposalType{ProposalTypeUpdateAdmin},
		},
		"multiple proper items": {
			input:     "StoreCode,InstantiateContract,MigrateContract",
			proposals: []ProposalType{ProposalTypeStoreCode, ProposalTypeInstantiateContract, ProposalTypeMigrateContract},
		},
		"empty trailing item": {
			input:   "StoreCode,",
			isError: true,
		},
		"invalid item": {
			input:   "StoreCode,InvalidProposalType",
			isError: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			chunks := strings.Split(tc.input, ",")
			proposals, err := ConvertToProposals(chunks)
			if tc.isError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, proposals, tc.proposals)
			}
		})
	}
}
