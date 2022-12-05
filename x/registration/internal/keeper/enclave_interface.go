package keeper

type EnclaveInterface interface {
	LoadSeed(masterCert []byte, seed []byte, apiKey []byte) ([]byte, error)
	GetEncryptedSeed(masterCert []byte) ([]byte, error)
}
