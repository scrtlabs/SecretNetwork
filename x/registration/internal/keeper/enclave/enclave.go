package enclave

import (
	"github.com/enigmampc/SecretNetwork/go-cosmwasm/api"
)

type API struct{}

func (API) LoadSeed(masterCert []byte, seed []byte) (bool, error) {
	return api.LoadSeedToEnclave(masterCert, seed)
}

func (API) GetEncryptedSeed(masterCert []byte) ([]byte, error) {
	return api.GetEncryptedSeed(masterCert)
}
