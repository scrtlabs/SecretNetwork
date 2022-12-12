package keeper

type EnclaveInterface interface {
	LoadSeed(masterKey []byte, seed []byte, apiKey []byte) (bool, error)
	GetEncryptedSeed(masterCert []byte) ([]byte, error)
}
