package types

// GenesisState is the struct representation of the export genesis
type GenesisState struct {
	Registration              []RegistrationNodeInfo `json:"reg_info"`
	NodeExchMasterCertificate []byte                 `json:"node_exch_cert"`
	IoMasterCertificate       []byte                 `json:"io_exch_cert"`
}

// ValidateGenesis performs basic validation of supply genesis data returning an
// error for any failed validation criteria.
func ValidateGenesis(data GenesisState) error {

	// todo: do we want to use this, or just fail if they don't exist?

	//if data.IoMasterCertificate == nil {
	//	return ErrCertificateInvalid
	//}
	//if data.NodeExchMasterCertificate == nil {
	//	return ErrCertificateInvalid
	//}

	return nil
}
