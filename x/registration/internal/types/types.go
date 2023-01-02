package types

import (
	"encoding/base64"
	"encoding/hex"
)

const (
	EnclaveRegistrationKey   = "new_node_seed_exchange_keypair.sealed"
	PublicKeyLength          = 64  // encoded length
	EncryptedKeyLength       = 192 // hex encoded length
	LegacyEncryptedKeyLength = 96  // hex encoded length
	MasterNodeKeyId          = "NodeExchMasterKey"
	MasterIoKeyId            = "IoExchMasterKey"
	SecretNodeSeedConfig     = "seed.json"
	SecretNodeCfgFolder      = ".node"
)

const (
	NodeExchMasterKeyPath = "node-master-key.txt"
	IoExchMasterKeyPath   = "io-master-key.txt"
	SeedPath              = "seed.txt"
	SeedConfigVersion     = 2
)

const AttestationCertPath = "attestation_cert.der"

type NodeID []byte

func (c SeedConfig) Decode() ([]byte, []byte, error) {
	enc, err := hex.DecodeString(c.EncryptedKey)
	if err != nil {
		return nil, nil, err
	}
	pk, err := base64.StdEncoding.DecodeString(c.MasterKey)
	if err != nil {
		return nil, nil, err
	}

	return pk, enc, nil
}

func (c LegacySeedConfig) Decode() ([]byte, []byte, error) {
	enc, err := hex.DecodeString(c.EncryptedKey)
	if err != nil {
		return nil, nil, err
	}
	pk, err := base64.StdEncoding.DecodeString(c.MasterCert)
	if err != nil {
		return nil, nil, err
	}

	return pk, enc, nil
}
