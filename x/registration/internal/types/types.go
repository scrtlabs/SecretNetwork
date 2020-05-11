package types

import (
	ra "github.com/enigmampc/EnigmaBlockchain/x/registration/internal/keeper/remote_attestation"
)

const PublicKeyLength = 128   // encoded length
const EncryptedKeyLength = 64 // encoded length

type NodeID []byte

type RegistrationNodeInfo struct {
	Certificate   ra.Certificate
	EncryptedSeed []byte
}
