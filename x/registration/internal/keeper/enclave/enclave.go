package enclave

import (
	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
)

type Api struct{}

func (Api) LoadSeed(masterCert []byte, seed []byte, apiKey []byte) ([]byte, error) {
	return api.LoadSeedToEnclave(masterCert, seed, apiKey)
}

func (Api) GetEncryptedSeed(masterCert []byte) ([]byte, error) {
	return api.GetEncryptedSeed(masterCert)
}
