//go:build nosgx
// +build nosgx

package api

// #include <stdlib.h>
// #include "bindings.h"
import "C"

import (
	"github.com/SigmaGmbH/librustgo/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"math/rand"
	"net"
)

// Value types
type (
	cint   = C.int
	cbool  = C.bool
	cusize = C.size_t
	cu8    = C.uint8_t
	cu32   = C.uint32_t
	cu64   = C.uint64_t
	ci8    = C.int8_t
	ci32   = C.int32_t
	ci64   = C.int64_t
)

// Pointers
type cu8_ptr = *C.uint8_t

// Connector is our custom connector
type Connector = types.Connector

func CheckNodeStatus() error {
	return nil
}
func RequestEpochKeys(host string, port int, isDCAP bool) error {
	return nil
}

// IsNodeInitialized checks if node was initialized and key manager state was sealed
func IsNodeInitialized() (bool, error) {
	return false, nil
}

// SetupSeedNode handles initialization of attestation server node which will share epoch keys with other nodes
func InitializeEnclave(shouldReset bool) error {
	return nil
}

// StartSeedServer handles initialization of attestation server
func StartSeedServer(addr string) error {
	return nil
}

func attestPeer(connection net.Conn) error {
	return nil
}

// RequestSeed handles request of seed from attestation server
func RequestSeed(hostname string, port int) error {
	return nil
}

// GetNodePublicKey handles request for node public key
func GetNodePublicKey(blockNumber uint64) (*types.NodePublicKeyResponse, error) {
	key := make([]byte, 32)
	rand.Read(key)
	return &types.NodePublicKeyResponse{PublicKey: key}, nil
}

// Call handles incoming call to contract or transfer of value
func Call(
	connector Connector,
	from, to, data, value []byte,
	accessList ethtypes.AccessList,
	gasLimit, nonce uint64,
	txContext *types.TransactionContext,
	commit bool,
	isUnencrypted bool,
) (*types.HandleTransactionResponse, error) {
	return nil, nil
}

// Create handles incoming request for creation of new contract
func Create(
	connector Connector,
	from, data, value []byte,
	accessList ethtypes.AccessList,
	gasLimit, nonce uint64,
	txContext *types.TransactionContext,
	commit bool,
) (*types.HandleTransactionResponse, error) {
	return nil, nil
}

// StartAttestationServer starts attestation server with 2 port (EPID and DCAP attestation)
func StartAttestationServer(epidAddress, dcapAddress string) error {
	return nil
}

func AddEpoch(startingBlock uint64) error {
	return nil
}

func RemoveLatestEpoch() error {
	return nil
}

func ListEpochs() ([]*types.EpochData, error) {
	return nil, nil
}
