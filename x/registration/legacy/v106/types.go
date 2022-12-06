package v106

const (
	ModuleName = "register"
)

type (
	Certificate []byte

	RegistrationNodeInfo struct {
		Certificate   Certificate
		EncryptedSeed []byte
	}

	// GenesisState is the struct representation of the export genesis
	GenesisState struct {
		Registration       []RegistrationNodeInfo `json:"reg_info"`
		NodeExchMasterCert []byte                 `json:"node_exch_cert"`
		IoMasterCert       []byte                 `json:"io_exch_cert"`
	}
)
