package librustgo

import (
	"github.com/SigmaGmbH/librustgo/internal/api"
	"github.com/SigmaGmbH/librustgo/types"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// Logs returned by EVM
type Log = types.Log
type Topic = types.Topic

// TransactionContext contains information about block timestamp, coinbase address, block gas limit, etc.
type TransactionContext = types.TransactionContext

// TransactionData contains data which is necessary to handle the transaction
type TransactionData = types.TransactionData

// Export protobuf messages for FFI
type QueryGetAccount = types.QueryGetAccount
type QueryGetAccountResponse = types.QueryGetAccountResponse
type CosmosRequest = types.CosmosRequest
type QueryInsertAccount = types.QueryInsertAccount
type QueryInsertAccountResponse = types.QueryInsertAccountResponse
type QueryContainsKey = types.QueryContainsKey
type QueryContainsKeyResponse = types.QueryContainsKeyResponse
type QueryGetAccountStorageCell = types.QueryGetAccountStorageCell
type QueryGetAccountStorageCellResponse = types.QueryGetAccountStorageCellResponse
type QueryGetAccountCode = types.QueryGetAccountCode
type QueryGetAccountCodeResponse = types.QueryGetAccountCodeResponse
type QueryInsertAccountCode = types.QueryInsertAccountCode
type QueryInsertAccountCodeResponse = types.QueryInsertAccountCodeResponse
type QueryInsertStorageCell = types.QueryInsertStorageCell
type QueryInsertStorageCellResponse = types.QueryInsertStorageCellResponse
type QueryRemove = types.QueryRemove
type QueryRemoveResponse = types.QueryRemoveResponse
type QueryRemoveStorageCell = types.QueryRemoveStorageCell
type QueryRemoveStorageCellResponse = types.QueryRemoveStorageCellResponse
type QueryBlockHash = types.QueryBlockHash
type QueryBlockHashResponse = types.QueryBlockHashResponse
type QueryAddVerificationDetails = types.QueryAddVerificationDetails
type QueryAddVerificationDetailsResponse = types.QueryAddVerificationDetailsResponse
type QueryHasVerification = types.QueryHasVerification
type QueryHasVerificationResponse = types.QueryHasVerificationResponse
type QueryGetVerificationData = types.QueryGetVerificationData
type VerificationDetails = types.VerificationDetails
type QueryGetVerificationDataResponse = types.QueryGetVerificationDataResponse

// Storage requests
type CosmosRequest_GetAccount = types.CosmosRequest_GetAccount
type CosmosRequest_InsertAccount = types.CosmosRequest_InsertAccount
type CosmosRequest_ContainsKey = types.CosmosRequest_ContainsKey
type CosmosRequest_AccountCode = types.CosmosRequest_AccountCode
type CosmosRequest_StorageCell = types.CosmosRequest_StorageCell
type CosmosRequest_InsertAccountCode = types.CosmosRequest_InsertAccountCode
type CosmosRequest_InsertStorageCell = types.CosmosRequest_InsertStorageCell
type CosmosRequest_Remove = types.CosmosRequest_Remove
type CosmosRequest_RemoveStorageCell = types.CosmosRequest_RemoveStorageCell
type CosmosRequest_AddVerificationDetails = types.CosmosRequest_AddVerificationDetails
type CosmosRequest_HasVerification = types.CosmosRequest_HasVerification
type CosmosRequest_GetVerificationData = types.CosmosRequest_GetVerificationData

// Backend requests
type CosmosRequest_BlockHash = types.CosmosRequest_BlockHash

type HandleTransactionResponse = types.HandleTransactionResponse
type NodePublicKeyRequest = types.NodePublicKeyRequest
type NodePublicKeyResponse = types.NodePublicKeyResponse

// CheckNodeStatus checks if SGX requirements are met
func CheckNodeStatus() error {
	return api.CheckNodeStatus()
}

// IsNodeInitialized checks if node was properly initialized and key manager state was sealed
func IsNodeInitialized() (bool, error) {
	return api.IsNodeInitialized()
}

// Call handles incoming transaction data to transfer value or call some contract
func Call(
	querier types.Connector,
	from, to, data, value []byte,
	accessList ethtypes.AccessList,
	gasLimit, nonce uint64,
	txContext *TransactionContext,
	commit bool,
	isUnencrypted bool,
) (*types.HandleTransactionResponse, error) {
	executionResult, err := api.Call(querier, from, to, data, value, accessList, gasLimit, nonce, txContext, commit, isUnencrypted)
	if err != nil {
		return &types.HandleTransactionResponse{}, err
	}

	return executionResult, nil
}

// Create handles incoming transaction data and creates a new smart contract
func Create(
	querier types.Connector,
	from, data, value []byte,
	accessList ethtypes.AccessList,
	gasLimit, nonce uint64,
	txContext *TransactionContext,
	commit bool,
) (*types.HandleTransactionResponse, error) {
	executionResult, err := api.Create(querier, from, data, value, accessList, gasLimit, nonce, txContext, commit)
	if err != nil {
		return &types.HandleTransactionResponse{}, err
	}

	return executionResult, nil
}

func InitializeEnclave(shouldReset bool) error {
	return api.InitializeEnclave(shouldReset)
}

// StartAttestationServer handles incoming request for starting attestation server
// to share epoch keys with new nodes which passed Remote Attestation.
func StartAttestationServer(epidAddress, dcapAddress string) error {
	return api.StartAttestationServer(epidAddress, dcapAddress)
}

// RequestSeed handles requesting seed and passing Remote Attestation.
// Returns error if Remote Attestation was not passed or provided seed server address is not accessible
func RequestEpochKeys(host string, port int, isDCAP bool) error {
	return api.RequestEpochKeys(host, port, isDCAP)
}

// GetNodePublicKey handles request for node public key
func GetNodePublicKey(blockNumber uint64) (*types.NodePublicKeyResponse, error) {
	result, err := api.GetNodePublicKey(blockNumber)
	if err != nil {
		return &types.NodePublicKeyResponse{}, err
	}
	return result, nil
}

// Libsgx_wrapperVersion returns the version of the loaded library
// at runtime. This can be used for debugging to verify the loaded version
// matches the expected version.
func Libsgx_wrapperVersion() (string, error) {
	return api.Libsgx_wrapperVersion()
}

func AddEpoch(startingBlock uint64) error {
	return api.AddEpoch(startingBlock)
}

func RemoveLatestEpoch() error {
	return api.RemoveLatestEpoch()
}

func ListEpochs() ([]*types.EpochData, error) {
	return api.ListEpochs()
}
