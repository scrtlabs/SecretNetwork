package mock

// To be able to run unit tests without needing the enclave

type EnclaveAPI struct{}

func (EnclaveAPI) LoadSeed(masterCert []byte, seed []byte) (bool, error) {
	return true, nil
}

func (EnclaveAPI) GetEncryptedSeed(masterCert []byte) ([]byte, error) {
	return []byte(""), nil
}
