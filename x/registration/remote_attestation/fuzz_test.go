// rename this to fuzz.go if you want to run the fuzzer

// +build gofuzz

package remote_attestation

func Fuzz(data []byte) int {
	if _, err := VerifyRaCert(data); err != nil {
		return 0
	}
	return 1
}
