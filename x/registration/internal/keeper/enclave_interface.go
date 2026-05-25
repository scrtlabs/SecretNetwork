package keeper

type EnclaveInterface interface {
	LoadSeed(masterKey []byte, seed []byte) (bool, error)
	GetEncryptedSeed(masterCert []byte, replace_machine_id []byte) ([]byte, []byte, error)
	GetEncryptedGenesisSeed(pk []byte) ([]byte, error)
}
