package enclave

import (
	"github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/api"
)

type EnclaveApi struct{}

func (EnclaveApi) LoadSeed(masterCert []byte, seed []byte) (bool, error) {
	return api.LoadSeedToEnclave(masterCert, seed)
}

func (EnclaveApi) GetEncryptedSeed(masterCert []byte) ([]byte, error) {
	return api.GetEncryptedSeed(masterCert)
}
