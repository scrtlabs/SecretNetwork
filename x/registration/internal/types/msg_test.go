package types

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestMsgRaAuthenticateRoute(t *testing.T) {
	addr1 := sdk.AccAddress([]byte("from"))
	cert, err := ioutil.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)
	// coins := sdk.NewCoins(sdk.NewInt64Coin("atom", 10))
	var msg = RaAuthenticate{
		addr1,
		cert,
	}

	require.Equal(t, msg.Route(), RouterKey)
	require.Equal(t, msg.Type(), "node-auth")
}

func TestMsgSendValidation(t *testing.T) {

	_ = os.Setenv("SGX_MODE", "SW")

	addr0 := sdk.AccAddress([]byte("qwlnmxj7prpx8rysxm2u"))

	cert, err := ioutil.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	certBadSig, err := ioutil.ReadFile("../../testdata/attestation_cert_invalid")
	require.NoError(t, err)

	invalidCert := []byte("aaaaaaaaaaa")

	// var emptyAddr sdk.AccAddress

	cases := []struct {
		valid bool
		tx    RaAuthenticate
	}{{true, RaAuthenticate{
		addr0,
		cert,
	}},
		// invalid address send
		{false, RaAuthenticate{
			addr0,
			invalidCert,
		}}, // malformed certificate
		{false, RaAuthenticate{
			addr0,
			certBadSig,
		}}, // certificate with a bad signature
	}

	for _, tc := range cases {
		err := tc.tx.ValidateBasic()
		if tc.valid {
			require.Nil(t, err)
		} else {
			require.NotNil(t, err)
		}
	}
}

func TestMsgSendGetSignBytes(t *testing.T) {
	addr0 := sdk.AccAddress([]byte("qwlnmxj7prpx8rysxm2u"))

	cert, err := ioutil.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	var msg = RaAuthenticate{
		addr0,
		cert,
	}
	res := msg.GetSignBytes()
	expected := `{"type":"reg/authenticate","value":{"ra_cert":"MIIBkzCCATmgAwIBAgIBATAKBggqhkjOPQQDAjAUMRIwEAYDVQQDDAlFbmlnbWFURUUwHhcNMjAwNTI1MDc1MzM0WhcNMjAwODIzMDc1MzM0WjAnMSUwIwYDVQQDDBxFbmlnbWFDaGFpbiBOb2RlIENlcnRpZmljYXRlMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEeG13Xxb1oWAqeBSahnmi8rEQH5Q3pGa+knDNikM7AIels1eqEpEebKV8RDxRlb4EdmAHPtxp5xVB6pDI/vh7wKNpMGcwZQYJYIZIAYb4QgENBFgwSEdyb3FpMjhIcFM1aFhNODNzZDZrL2lJbGdjckZjM3IrTmpHa2R3VU16ZEJCQnFZKzd5ZXg4c2V1eERaeG9lb1JmS0l6R0xZMDMrVVdrZzl2K3V5UT09MAoGCCqGSM49BAMCA0gAMEUCIFCpcWt77lCX+I8WpuRpkGdHYSp/KeCM5lEbfkls/VolAiEAulO7Btux2jcE8QP3Mo9/7cGm/BykxZxAbJIjO9AqLHY=","sender":"cosmos1w9mkcmnd0p4rwurjwpursunewdux6vn4d4tp6g"}}`
	require.Equal(t, expected, string(res))
}

func TestMsgSendGetSigners(t *testing.T) {
	addr0 := sdk.AccAddress([]byte("qwlnmxj7prpx8rysxm2u"))

	cert, err := ioutil.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	var msg = RaAuthenticate{
		addr0,
		cert,
	}
	res := msg.GetSigners()
	// TODO: fix this !
	require.Equal(t, fmt.Sprintf("%v", res), "[71776C6E6D786A377072707838727973786D3275]")
}
