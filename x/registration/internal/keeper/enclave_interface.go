package keeper

type EnclaveInterface interface {
	LoadSeed(masterKey []byte, seed []byte, apiKey []byte) ([]byte, error)
	GetEncryptedSeed(masterCert []byte) ([]byte, error)
}
