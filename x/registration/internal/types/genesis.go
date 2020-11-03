package types

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
