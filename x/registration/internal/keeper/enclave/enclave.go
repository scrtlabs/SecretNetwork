package enclave

import (
	"github.com/enigmampc/SecretNetwork/go-cosmwasm/api"
)

type Api struct{}

func (Api) LoadSeed(masterCert []byte, seed []byte) (bool, error) {
	return api.LoadSeedToEnclave(masterCert, seed)
}

func (Api) GetEncryptedSeed(masterCert []byte) ([]byte, error) {
	return api.GetEncryptedSeed(masterCert)
}
