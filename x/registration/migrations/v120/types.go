package v120

const (
	ModuleName = "register"
)

type (
	Certificate []byte

	RegistrationNodeInfo struct {
		Certificate   Certificate `json:"certificate"`
		EncryptedSeed []byte      `json:"encrypted_seed"`
	}

	MasterCertificate struct {
		Bytes []byte `json:"bytes"`
	}

	// GenesisState is the struct representation of the export genesis
	GenesisState struct {
		Registration              []*RegistrationNodeInfo `json:"registration"`
		NodeExchMasterCertificate *MasterCertificate      `json:"node_exch_master_certificate"`
		IoMasterCertificate       *MasterCertificate      `json:"io_master_certificate"`
	}
)
