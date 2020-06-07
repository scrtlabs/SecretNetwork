package keeper

import (
	"github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/api"
)

type EnclaveInterface interface {
	LoadSeed(masterCert []byte, seed []byte) (bool, error)
	GetEncryptedSeed(masterCert []byte) ([]byte, error)
}

type EnclaveApi struct{}

func (EnclaveApi) LoadSeed(masterCert []byte, seed []byte) (bool, error) {
	return api.LoadSeedToEnclave(masterCert, seed)
}

func (EnclaveApi) GetEncryptedSeed(masterCert []byte) ([]byte, error) {
	return api.GetEncryptedSeed(masterCert)
}

// To be able to run unit tests without needing the enclave

type MockEnclaveApi struct{}

func (MockEnclaveApi) LoadSeed(masterCert []byte, seed []byte) (bool, error) {
	return true, nil
}

func (MockEnclaveApi) GetEncryptedSeed(masterCert []byte) ([]byte, error) {
	return []byte(""), nil
}
