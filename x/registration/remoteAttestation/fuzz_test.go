// rename this to fuzz.go if you want to run the fuzzer

//go:build gofuzz
// +build gofuzz

package remoteAttestation

func Fuzz(data []byte) int {
	if _, err := VerifyRaCert(data); err != nil {
		return 0
	}
	return 1
}
