package keeper

type EnclaveInterface interface {
	LoadSeed(masterCert []byte, seed []byte, apiKey []byte) (bool, error)
	GetEncryptedSeed(masterCert []byte) ([]byte, error)
}
