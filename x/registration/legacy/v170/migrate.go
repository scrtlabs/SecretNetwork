package v170

import (
	v170rk "github.com/scrtlabs/SecretNetwork/x/registration/internal/keeper"
	v170registration "github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
	v120registration "github.com/scrtlabs/SecretNetwork/x/registration/legacy/v120"
	v170ra "github.com/scrtlabs/SecretNetwork/x/registration/remote_attestation"
)

// Migrate accepts exported v1.2.0 x/registration genesis state and
// migrates it to v1.7.0 x/registration genesis state. The migration includes:
//
// - Re-encode in v1.7.0 GenesisState.
func Migrate(regGenState v120registration.GenesisState) *v170registration.GenesisState {
	registrations := make([]*v170registration.RegistrationNodeInfo, len(regGenState.Registration))
	for i, regNodeInfo := range regGenState.Registration {
		registrations[i] = &v170registration.RegistrationNodeInfo{
			Certificate:   v170ra.Certificate(regNodeInfo.Certificate),
			EncryptedSeed: regNodeInfo.EncryptedSeed,
		}
	}

	nodeExchMasterKey, err := v170rk.FetchRawPubKeyFromLegacyCert(regGenState.NodeExchMasterCertificate.Bytes)
	if err != nil {
		return nil
	}

	iohMasterKey, err := v170rk.FetchRawPubKeyFromLegacyCert(regGenState.IoMasterCertificate.Bytes)
	if err != nil {
		return nil
	}

	return &v170registration.GenesisState{
		Registration: registrations,
		NodeExchMasterKey: &v170registration.MasterKey{
			Bytes: nodeExchMasterKey,
		},
		IoMasterKey: &v170registration.MasterKey{
			Bytes: iohMasterKey,
		},
	}
}
