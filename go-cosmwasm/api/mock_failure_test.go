package api

import "fmt"

/***** Mock GoAPI ****/

func MockFailureCanonicalAddress(human string) ([]byte, uint64, error) {
	return nil, 0, fmt.Errorf("mock failure - canonical_address")
}

func MockFailureHumanAddress(canon []byte) (string, uint64, error) {
	return "", 0, fmt.Errorf("mock failure - human_address")
}

func NewMockFailureAPI() *GoAPI {
	return &GoAPI{
		HumanAddress:     MockFailureHumanAddress,
		CanonicalAddress: MockFailureCanonicalAddress,
	}
}
