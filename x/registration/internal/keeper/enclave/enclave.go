package enclave

import (
	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
)

type Api struct{}

func (Api) LoadSeed(masterKey []byte, seed []byte) (bool, error) {
	return api.LoadSeedToEnclave(masterKey, seed)
}

func (Api) GetEncryptedSeed(masterCert []byte, replace_machine_id []byte) ([]byte, []byte, error) {
	return api.GetEncryptedSeed(masterCert, replace_machine_id)
}

func (Api) GetEncryptedGenesisSeed(pk []byte) ([]byte, error) {
	return api.GetEncryptedGenesisSeed(pk)
}
