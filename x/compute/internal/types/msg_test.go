package types

import (
	"bytes"
	"strings"
	"testing"

	sdk "github.com/enigmampc/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilderRegexp(t *testing.T) {
	cases := map[string]struct {
		example string
		valid   bool
	}{
		"normal":                {"fedora/httpd:version1.0", true},
		"another valid org":     {"confio/js-builder-1:test", true},
		"no org name":           {"cosmwasm-opt:0.6.3", false},
		"invalid trailing char": {"someone/cosmwasm-opt-:0.6.3", false},
		"invalid leading char":  {"confio/.builder-1:manual", false},
		"multiple orgs":         {"confio/assembly-script/optimizer:v0.9.1", true},
		"too long":              {"over-128-character-limit/some-long-sub-path/and-yet-another-long-name/testtesttesttesttesttesttest/foobarfoobar/foobarfoobar:randomstringrandomstringrandomstringrandomstring", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateBuilder(tc.example)
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})

	}
}

func TestStoreCodeValidation(t *testing.T) {
	badAddress, err := sdk.AccAddressFromHex("012345")
	require.NoError(t, err)
	// proper address size
	goodAddress := sdk.AccAddress(make([]byte, 20))

	cases := map[string]struct {
		msg   MsgStoreCode
		valid bool
	}{
		"empty": {
			msg:   MsgStoreCode{},
			valid: false,
		},
		"correct minimal": {
			msg: MsgStoreCode{
				Sender:       goodAddress,
				WASMByteCode: []byte("foo"),
			},
			valid: true,
		},
		"missing code": {
			msg: MsgStoreCode{
				Sender: goodAddress,
			},
			valid: false,
		},
		"bad sender minimal": {
			msg: MsgStoreCode{
				Sender:       badAddress,
				WASMByteCode: []byte("foo"),
			},
			valid: false,
		},
		"correct maximal": {
			msg: MsgStoreCode{
				Sender:       goodAddress,
				WASMByteCode: []byte("foo"),
				Builder:      "confio/cosmwasm-opt:0.6.2",
				Source:       "https://crates.io/api/v1/crates/cw-erc20/0.1.0/download",
			},
			valid: true,
		},
		"invalid builder": {
			msg: MsgStoreCode{
				Sender:       goodAddress,
				WASMByteCode: []byte("foo"),
				Builder:      "-bad-opt:0.6.2",
				Source:       "https://crates.io/api/v1/crates/cw-erc20/0.1.0/download",
			},
			valid: false,
		},
		"invalid source scheme": {
			msg: MsgStoreCode{
				Sender:       goodAddress,
				WASMByteCode: []byte("foo"),
				Builder:      "cosmwasm-opt:0.6.2",
				Source:       "ftp://crates.io/api/download.tar.gz",
			},
			valid: false,
		},
		"invalid source format": {
			msg: MsgStoreCode{
				Sender:       goodAddress,
				WASMByteCode: []byte("foo"),
				Builder:      "cosmwasm-opt:0.6.2",
				Source:       "/api/download-ss",
			},
			valid: false,
		},
		"invalid InstantiatePermission": {
			msg: MsgStoreCode{
				Sender:                goodAddress,
				WASMByteCode:          []byte("foo"),
				InstantiatePermission: &AccessConfig{Type: OnlyAddress, Address: badAddress},
			},
			valid: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestInstantiateContractValidation(t *testing.T) {
	badAddress, err := sdk.AccAddressFromHex("012345")
	require.NoError(t, err)
	// proper address size
	goodAddress := sdk.AccAddress(make([]byte, 20))

	cases := map[string]struct {
		msg   MsgInstantiateContract
		valid bool
	}{
		"empty": {
			msg:   MsgInstantiateContract{},
			valid: false,
		},
		"correct minimal": {
			msg: MsgInstantiateContract{
				Sender:  goodAddress,
				CodeID:  1,
				Label:   "foo",
				InitMsg: []byte("{}"),
			},
			valid: true,
		},
		"missing code": {
			msg: MsgInstantiateContract{
				Sender:  goodAddress,
				Label:   "foo",
				InitMsg: []byte("{}"),
			},
			valid: false,
		},
		"missing label": {
			msg: MsgInstantiateContract{
				Sender:  goodAddress,
				InitMsg: []byte("{}"),
			},
			valid: false,
		},
		"label too long": {
			msg: MsgInstantiateContract{
				Sender: goodAddress,
				Label:  strings.Repeat("food", 33),
			},
			valid: false,
		},
		"bad sender minimal": {
			msg: MsgInstantiateContract{
				Sender:  badAddress,
				CodeID:  1,
				Label:   "foo",
				InitMsg: []byte("{}"),
			},
			valid: false,
		},
		"correct maximal": {
			msg: MsgInstantiateContract{
				Sender:    goodAddress,
				CodeID:    1,
				Label:     "foo",
				InitMsg:   []byte(`{"some": "data"}`),
				InitFunds: sdk.Coins{sdk.Coin{Denom: "foobar", Amount: sdk.NewInt(200)}},
			},
			valid: true,
		},
		"negative funds": {
			msg: MsgInstantiateContract{
				Sender:  goodAddress,
				CodeID:  1,
				Label:   "foo",
				InitMsg: []byte(`{"some": "data"}`),
				// we cannot use sdk.NewCoin() constructors as they panic on creating invalid data (before we can test)
				InitFunds: sdk.Coins{sdk.Coin{Denom: "foobar", Amount: sdk.NewInt(-200)}},
			},
			valid: false,
		},
		"non json init msg": {
			msg: MsgInstantiateContract{
				Sender:  goodAddress,
				CodeID:  1,
				Label:   "foo",
				InitMsg: []byte("invalid-json"),
			},
			valid: false,
		},
		"empty init msg": {
			msg: MsgInstantiateContract{
				Sender: goodAddress,
				CodeID: 1,
				Label:  "foo",
			},
			valid: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestExecuteContractValidation(t *testing.T) {
	badAddress, err := sdk.AccAddressFromHex("012345")
	require.NoError(t, err)
	// proper address size
	goodAddress := sdk.AccAddress(make([]byte, 20))

	cases := map[string]struct {
		msg   MsgExecuteContract
		valid bool
	}{
		"empty": {
			msg:   MsgExecuteContract{},
			valid: false,
		},
		"correct minimal": {
			msg: MsgExecuteContract{
				Sender:   goodAddress,
				Contract: goodAddress,
				Msg:      []byte("{}"),
			},
			valid: true,
		},
		"correct all": {
			msg: MsgExecuteContract{
				Sender:    goodAddress,
				Contract:  goodAddress,
				Msg:       []byte(`{"some": "data"}`),
				SentFunds: sdk.Coins{sdk.Coin{Denom: "foobar", Amount: sdk.NewInt(200)}},
			},
			valid: true,
		},
		"bad sender": {
			msg: MsgExecuteContract{
				Sender:   badAddress,
				Contract: goodAddress,
				Msg:      []byte(`{"some": "data"}`),
			},
			valid: false,
		},
		"empty sender": {
			msg: MsgExecuteContract{
				Contract: goodAddress,
				Msg:      []byte(`{"some": "data"}`),
			},
			valid: false,
		},
		"bad contract": {
			msg: MsgExecuteContract{
				Sender:   goodAddress,
				Contract: badAddress,
				Msg:      []byte(`{"some": "data"}`),
			},
			valid: false,
		},
		"empty contract": {
			msg: MsgExecuteContract{
				Sender: goodAddress,
				Msg:    []byte(`{"some": "data"}`),
			},
			valid: false,
		},
		"negative funds": {
			msg: MsgExecuteContract{
				Sender:    goodAddress,
				Contract:  goodAddress,
				Msg:       []byte(`{"some": "data"}`),
				SentFunds: sdk.Coins{sdk.Coin{Denom: "foobar", Amount: sdk.NewInt(-1)}},
			},
			valid: false,
		},
		"duplicate funds": {
			msg: MsgExecuteContract{
				Sender:    goodAddress,
				Contract:  goodAddress,
				Msg:       []byte(`{"some": "data"}`),
				SentFunds: sdk.Coins{sdk.Coin{Denom: "foobar", Amount: sdk.NewInt(1)}, sdk.Coin{Denom: "foobar", Amount: sdk.NewInt(1)}},
			},
			valid: false,
		},
		"non json msg": {
			msg: MsgExecuteContract{
				Sender:   goodAddress,
				Contract: goodAddress,
				Msg:      []byte("invalid-json"),
			},
			valid: false,
		},
		"empty msg": {
			msg: MsgExecuteContract{
				Sender:   goodAddress,
				Contract: goodAddress,
			},
			valid: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestMsgUpdateAdministrator(t *testing.T) {
	badAddress, err := sdk.AccAddressFromHex("012345")
	require.NoError(t, err)
	// proper address size
	goodAddress := sdk.AccAddress(make([]byte, 20))
	otherGoodAddress := sdk.AccAddress(bytes.Repeat([]byte{0x1}, 20))
	anotherGoodAddress := sdk.AccAddress(bytes.Repeat([]byte{0x2}, 20))

	specs := map[string]struct {
		src    MsgUpdateAdmin
		expErr bool
	}{
		"all good": {
			src: MsgUpdateAdmin{
				Sender:   goodAddress,
				NewAdmin: otherGoodAddress,
				Contract: anotherGoodAddress,
			},
		},
		"new admin required": {
			src: MsgUpdateAdmin{
				Sender:   goodAddress,
				Contract: anotherGoodAddress,
			},
			expErr: true,
		},
		"bad sender": {
			src: MsgUpdateAdmin{
				Sender:   badAddress,
				NewAdmin: otherGoodAddress,
				Contract: anotherGoodAddress,
			},
			expErr: true,
		},
		"bad new admin": {
			src: MsgUpdateAdmin{
				Sender:   goodAddress,
				NewAdmin: badAddress,
				Contract: anotherGoodAddress,
			},
			expErr: true,
		},
		"bad contract addr": {
			src: MsgUpdateAdmin{
				Sender:   goodAddress,
				NewAdmin: otherGoodAddress,
				Contract: badAddress,
			},
			expErr: true,
		},
		"new admin same as old admin": {
			src: MsgUpdateAdmin{
				Sender:   goodAddress,
				NewAdmin: goodAddress,
				Contract: anotherGoodAddress,
			},
			expErr: true,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			err := spec.src.ValidateBasic()
			if spec.expErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMsgClearAdministrator(t *testing.T) {
	badAddress, err := sdk.AccAddressFromHex("012345")
	require.NoError(t, err)
	// proper address size
	goodAddress := sdk.AccAddress(make([]byte, 20))
	anotherGoodAddress := sdk.AccAddress(bytes.Repeat([]byte{0x2}, 20))

	specs := map[string]struct {
		src    MsgClearAdmin
		expErr bool
	}{
		"all good": {
			src: MsgClearAdmin{
				Sender:   goodAddress,
				Contract: anotherGoodAddress,
			},
		},
		"bad sender": {
			src: MsgClearAdmin{
				Sender:   badAddress,
				Contract: anotherGoodAddress,
			},
			expErr: true,
		},
		"bad contract addr": {
			src: MsgClearAdmin{
				Sender:   goodAddress,
				Contract: badAddress,
			},
			expErr: true,
		},
		"contract missing": {
			src: MsgClearAdmin{
				Sender: goodAddress,
			},
			expErr: true,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			err := spec.src.ValidateBasic()
			if spec.expErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMsgMigrateContract(t *testing.T) {
	badAddress, err := sdk.AccAddressFromHex("012345")
	require.NoError(t, err)
	// proper address size
	goodAddress := sdk.AccAddress(make([]byte, 20))
	anotherGoodAddress := sdk.AccAddress(bytes.Repeat([]byte{0x2}, 20))

	specs := map[string]struct {
		src    MsgMigrateContract
		expErr bool
	}{
		"all good": {
			src: MsgMigrateContract{
				Sender:     goodAddress,
				Contract:   anotherGoodAddress,
				CodeID:     1,
				MigrateMsg: []byte("{}"),
			},
		},
		"bad sender": {
			src: MsgMigrateContract{
				Sender:   badAddress,
				Contract: anotherGoodAddress,
				CodeID:   1,
			},
			expErr: true,
		},
		"empty sender": {
			src: MsgMigrateContract{
				Contract: anotherGoodAddress,
				CodeID:   1,
			},
			expErr: true,
		},
		"empty code": {
			src: MsgMigrateContract{
				Sender:   goodAddress,
				Contract: anotherGoodAddress,
			},
			expErr: true,
		},
		"bad contract addr": {
			src: MsgMigrateContract{
				Sender:   goodAddress,
				Contract: badAddress,
				CodeID:   1,
			},
			expErr: true,
		},
		"empty contract addr": {
			src: MsgMigrateContract{
				Sender: goodAddress,
				CodeID: 1,
			},
			expErr: true,
		},
		"non json migrateMsg": {
			src: MsgMigrateContract{
				Sender:     goodAddress,
				Contract:   anotherGoodAddress,
				CodeID:     1,
				MigrateMsg: []byte("invalid json"),
			},
			expErr: true,
		},
		"empty migrateMsg": {
			src: MsgMigrateContract{
				Sender:   goodAddress,
				Contract: anotherGoodAddress,
				CodeID:   1,
			},
			expErr: true,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			err := spec.src.ValidateBasic()
			if spec.expErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
