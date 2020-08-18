package types

import (
	"encoding/json"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateParams(t *testing.T) {
	var (
		anyAddress     = make([]byte, sdk.AddrLen)
		invalidAddress = make([]byte, sdk.AddrLen-1)
	)

	specs := map[string]struct {
		src    Params
		expErr bool
	}{
		"all good with defaults": {
			src: DefaultParams(),
		},
		"all good with nobody": {
			src: Params{
				UploadAccess:                 AllowNobody,
				DefaultInstantiatePermission: Nobody,
			},
		},
		"all good with everybody": {
			src: Params{
				UploadAccess:                 AllowEverybody,
				DefaultInstantiatePermission: Everybody,
			},
		},
		"all good with only address": {
			src: Params{
				UploadAccess:                 OnlyAddress.With(anyAddress),
				DefaultInstantiatePermission: OnlyAddress,
			},
		},
		"reject empty type in instantiate permission": {
			src: Params{
				UploadAccess:                 AllowNobody,
				DefaultInstantiatePermission: "",
			},
			expErr: true,
		},
		"reject unknown type in instantiate": {
			src: Params{
				UploadAccess:                 AllowNobody,
				DefaultInstantiatePermission: "Undefined",
			},
			expErr: true,
		},
		"reject invalid address in only address": {
			src: Params{
				UploadAccess:                 AccessConfig{Type: OnlyAddress, Address: invalidAddress},
				DefaultInstantiatePermission: OnlyAddress,
			},
			expErr: true,
		},
		"reject UploadAccess Everybody with obsolete address": {
			src: Params{
				UploadAccess:                 AccessConfig{Type: Everybody, Address: anyAddress},
				DefaultInstantiatePermission: OnlyAddress,
			},
			expErr: true,
		},
		"reject UploadAccess Nobody with obsolete address": {
			src: Params{
				UploadAccess:                 AccessConfig{Type: Nobody, Address: anyAddress},
				DefaultInstantiatePermission: OnlyAddress,
			},
			expErr: true,
		},
		"reject empty UploadAccess": {
			src: Params{
				DefaultInstantiatePermission: OnlyAddress,
			},
			expErr: true,
		}, "reject undefined permission in UploadAccess": {
			src: Params{
				UploadAccess:                 AccessConfig{Type: Undefined},
				DefaultInstantiatePermission: OnlyAddress,
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

func TestAccessTypeMarshalJson(t *testing.T) {
	specs := map[string]struct {
		src AccessType
		exp string
	}{
		"Undefined":   {src: Undefined, exp: `"Undefined"`},
		"Nobody":      {src: Nobody, exp: `"Nobody"`},
		"OnlyAddress": {src: OnlyAddress, exp: `"OnlyAddress"`},
		"Everybody":   {src: Everybody, exp: `"Everybody"`},
		"unknown":     {src: "", exp: `"Undefined"`},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			got, err := json.Marshal(spec.src)
			require.NoError(t, err)
			assert.Equal(t, []byte(spec.exp), got)
		})
	}
}
func TestAccessTypeUnMarshalJson(t *testing.T) {
	specs := map[string]struct {
		src string
		exp AccessType
	}{
		"Undefined":   {src: `"Undefined"`, exp: Undefined},
		"Nobody":      {src: `"Nobody"`, exp: Nobody},
		"OnlyAddress": {src: `"OnlyAddress"`, exp: OnlyAddress},
		"Everybody":   {src: `"Everybody"`, exp: Everybody},
		"unknown":     {src: `""`, exp: Undefined},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			var got AccessType
			err := json.Unmarshal([]byte(spec.src), &got)
			require.NoError(t, err)
			assert.Equal(t, spec.exp, got)
		})
	}
}
