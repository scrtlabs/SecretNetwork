package v120

const (
	ModuleName = "register"
)

type (
	Certificate []byte

	RegistrationNodeInfo struct {
		Certificate   Certificate
		EncryptedSeed []byte
	}

	MasterCertificate struct {
		Bytes []byte
	}

	// GenesisState is the struct representation of the export genesis
	GenesisState struct {
		Registration              []*RegistrationNodeInfo
		NodeExchMasterCertificate *MasterCertificate
		IoMasterCertificate       *MasterCertificate
	}
)
