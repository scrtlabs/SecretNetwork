package mock

// To be able to run unit tests without needing the enclave

type MockEnclaveApi struct{} //nolint:revive

func (MockEnclaveApi) LoadSeed(masterCert []byte, seed []byte, apiKey []byte) (bool, error) {
	return true, nil
}

func (MockEnclaveApi) GetEncryptedSeed(masterCert []byte) ([]byte, error) {
	return []byte(""), nil
}
