package remote_attestation

import (
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
)

func Test_ValidateCertificateHwMode(t *testing.T) {
	cert, err := ioutil.ReadFile("../testdata/attestation_cert_hw_v2")
	require.NoError(t, err)
	_ = os.Setenv("SGX_MODE", "HW")
	_, err = VerifyRaCert(cert)
	require.NoError(t, err)
}

func Test_ValidateCertificateSwMode(t *testing.T) {
	cert, err := ioutil.ReadFile("../testdata/attestation_cert_sw")
	require.NoError(t, err)
	_ = os.Setenv("SGX_MODE", "SW")
	_, err = VerifyRaCert(cert)
	require.NoError(t, err)
}

func Test_InvalidCertificate(t *testing.T) {
	cert, err := ioutil.ReadFile("../testdata/attestation_cert_invalid")
	require.NoError(t, err)

	_, err = VerifyRaCert(cert)
	require.Error(t, err)
}

func Test_InvalidRandomDataAsCert(t *testing.T) {
	cert := []byte("Here is a string....")

	_, err := VerifyRaCert(cert)
	require.Error(t, err)
}

func Test_FuzzCrashers(t *testing.T) {

	var crashers = [][]byte{
		[]byte("\x06\b*\x86H\xce=\x03\x01\a0\xd80r0"),
		[]byte("\x06\b*\x86H\xce=\x03\x01\a\f\x1cEnigmaCh" +
			"ain Node Ce000000000"),
		[]byte("\x06\b*\x86H\xce=\x03\x01\a00\x06%0#\x06a|e" +
			"0Y0\xbd7\x04�\xbd\x06H�=\x02\x01dn" +
			"smessage.00000000000"),
		[]byte("\x06\b*\x86H\xce=\x03\x01\a00: failed" +
			" to load system root" +
			"s and n0000000000000"),
		[]byte("00000000000\x06\b*\x86H\xce=\x03\x01" +
			"\a0"),
		[]byte("00000000000000000000" +
			"000\x06\b*\x86H\xce=\x03\x01\a0\t00000" +
			"0000000000000000000"),
		[]byte("000000000000000000\x06\b" +
			"*\x86H\xce=\x03\x01\a0"),
		[]byte("00000000000000000000" +
			"000\x06\b*\x86H\xce=\x03\x01\a\x06\t`\x86H\x01\x86" +
			"\xf8B\x01\r0\xff\xff\u007f000000000000" +
			"00000000000000000000" +
			"0000000000000000000"),
		[]byte("00000000000\xbd0"),
		[]byte("00000000000\x0600000|0"),
	}

	_ = os.Setenv("SGX_MODE", "HW")

	for _, f := range crashers {
		_, _ = VerifyRaCert(f)
	}
}
