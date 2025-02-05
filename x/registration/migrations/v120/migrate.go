package v120

import (
	v106registration "github.com/scrtlabs/SecretNetwork/x/registration/migrations/v106"
)

// Migrate accepts exported v1.0.6 x/registration genesis state and
// migrates it to v1.2.0 x/registration genesis state. The migration includes:
//
// - Re-encode in v1.2.0 GenesisState.
func Migrate(regGenState v106registration.GenesisState) *GenesisState {
	registrations := make([]*RegistrationNodeInfo, len(regGenState.Registration))
	for i, regNodeInfo := range regGenState.Registration {
		registrations[i] = &RegistrationNodeInfo{
			Certificate:   Certificate(regNodeInfo.Certificate),
			EncryptedSeed: regNodeInfo.EncryptedSeed,
		}
	}

	return &GenesisState{
		Registration: registrations,
		NodeExchMasterCertificate: &MasterCertificate{
			Bytes: regGenState.NodeExchMasterCertificate,
		},
		IoMasterCertificate: &MasterCertificate{
			Bytes: regGenState.IoMasterCertificate,
		},
	}
}
