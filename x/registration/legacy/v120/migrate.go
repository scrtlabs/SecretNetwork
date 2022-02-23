package v120

import (
	v120registration "github.com/enigmampc/SecretNetwork/x/registration/internal/types"
	v106registration "github.com/enigmampc/SecretNetwork/x/registration/legacy/v106"
	v120ra "github.com/enigmampc/SecretNetwork/x/registration/remote_attestation"
)

// Migrate accepts exported v1.0.6 x/registration genesis state and
// migrates it to v1.2.0 x/registration genesis state. The migration includes:
//
// - Re-encode in v1.2.0 GenesisState.
func Migrate(regGenState v106registration.GenesisState) *v120registration.GenesisState {
	registrations := make([]*v120registration.RegistrationNodeInfo, len(regGenState.Registration))
	for i, regNodeInfo := range regGenState.Registration {
		registrations[i] = &v120registration.RegistrationNodeInfo{
			Certificate:   v120ra.Certificate(regNodeInfo.Certificate),
			EncryptedSeed: regNodeInfo.EncryptedSeed,
		}
	}

	return &v120registration.GenesisState{
		Registration: registrations,
		NodeExchMasterCertificate: &v120registration.MasterCertificate{
			Bytes: regGenState.NodeExchMasterCertificate,
		},
		IoMasterCertificate: &v120registration.MasterCertificate{
			Bytes: regGenState.IoMasterCertificate,
		},
	}
}
