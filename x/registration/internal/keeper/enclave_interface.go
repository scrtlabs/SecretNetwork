package keeper

type EnclaveInterface interface {
	LoadSeed(masterCert []byte, seed []byte) (bool, error)
	GetEncryptedSeed(masterCert []byte) ([]byte, error)
}
