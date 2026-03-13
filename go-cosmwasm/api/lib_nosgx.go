//go:build !secretcli && nosgx

package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	v1types "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Cache struct{}

func HealthCheck() ([]byte, error) {
	return []byte("replay"), nil
}

func InitBootstrap(spid []byte, apiKey []byte) ([]byte, error) {
	return nil, nil
}

func SubmitBlockSignatures(header []byte, commit []byte, txs []byte, encRandom []byte, cronMsgs []byte) ([]byte, []byte, error) {
	return nil, nil, errors.New("submit block signatures not supported on non-SGX node")
}

func SubmitValidatorSetEvidence(evidence []byte) error {
	logInfo("SubmitValidatorSetEvidence", "Skipped in replay mode")
	return nil
}

func LoadSeedToEnclave(masterKey []byte, seed []byte, apiKey []byte) (bool, error) {
	return true, nil
}

type Querier = types.Querier

func MigrationOp(op uint32) (bool, error) {
	logInfo("MigrationOp", "Skipped in replay mode")
	return true, nil // no-op success so upgrade handlers don't fail
}

func RotateStore(kvs []byte) (bool, error) {
	return false, errors.New("RotateStore not supported on non-SGX node")
}

func EmergencyApproveUpgrade(nodeDir string, msg string) (bool, error) {
	return false, errors.New("EmergencyApproveUpgrade not supported on non-SGX node")
}

func InitCache(dataDir string, supportedFeatures string, cacheSize uint64) (Cache, error) {
	return Cache{}, nil
}

func ReleaseCache(cache Cache) {
}

func InitEnclaveRuntime(ModuleCacheSize uint16) error {
	return nil
}

func Create(cache Cache, wasm []byte) ([]byte, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	wasmHash := sha256.Sum256(wasm)

	// Attempt to replay recorded Create result from SGX node
	if recorder.IsReplayMode() {
		codeHash, errMsg, found := recorder.ReplayCreateResult(height, wasmHash[:])
		if found {
			if errMsg != "" {
				return nil, fmt.Errorf("%s", errMsg)
			}
			return codeHash, nil
		}

		// Not found locally — fetch all Create results for this block from remote SGX node
		client := GetEcallClient()
		if client != nil && client.IsConnected() {
			maxRetries := 20
			retryDelay := 50 * time.Millisecond
			maxDelay := 2 * time.Second

			for attempt := 0; attempt < maxRetries; attempt++ {
				results, wasmHashes, err := client.FetchBlockCreateResults(height)
				if err == nil && len(results) > 0 {
					// Match wasmHash directly from fetched results (no DB round-trip needed)
					for i, fetchedHash := range wasmHashes {
						if bytes.Equal(fetchedHash, wasmHash[:]) {
							r := results[i]
							logInfo("Create", "Matched Create result from SGX node: height=%d hasError=%v (attempt %d)", height, r.HasError, attempt+1)
							if r.HasError {
								return nil, fmt.Errorf("%s", r.ErrorMsg)
							}
							return r.CodeHash, nil
						}
					}
				}

				if attempt < maxRetries-1 {
					delay := retryDelay * time.Duration(1<<uint(attempt))
					if delay > maxDelay {
						delay = maxDelay
					}
					time.Sleep(delay)
				}
			}
			logWarn("Create", "Create result NOT FOUND after retries: height=%d wasmHash=%x", height, wasmHash[:8])
		}
	}

	// Known failed MsgStoreCode transactions (SGX rejected these).
	// Without a recording, we must reject them here to match consensus.
	switch height {
	case 23570428, // tx 00B33CC5..., "Unknown opcode 252"
		23570538,  // tx FDD8213C...
		23570819,  // tx C0F65C56...
		23571940,  // tx 30043C55...
		23572126:  // tx ACCDBCFA...
		return nil, fmt.Errorf("Error during static Wasm validation: Wasm bytecode could not be deserialized. Deserialization error: \"Unknown opcode 252\"")
	}

	return wasmHash[:], nil
}

func GetCode(cache Cache, code_id []byte) ([]byte, error) {
	return nil, errors.New("GetCode is not supported on non-SGX node")
}

func Migrate(
	cache Cache,
	code_id []byte,
	params []byte,
	msg []byte,
	gasMeter *GasMeter,
	store KVStore,
	api *GoAPI,
	querier *Querier,
	gasLimit uint64,
	sigInfo []byte,
	admin []byte,
	adminProof []byte,
) ([]byte, uint64, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	execIndex := recorder.NextExecutionIndex()

	logDebug("Migrate", "REPLAY mode: height=%d execIndex=%d", height, execIndex)
	if result, gas, err, found := replayExecution(store, gasMeter, execIndex); found {
		logDebug("Migrate", "REPLAY success: resultLen=%d gas=%d err=%v", len(result), gas, err)
		return result, gas, err
	}
	logWarn("Migrate", "REPLAY FAILED: trace not found!")
	return nil, 0, fmt.Errorf("Migrate replay failed: trace not found for height %d index %d", height, execIndex)
}

func UpdateAdmin(
	cache Cache,
	code_id []byte,
	params []byte,
	gasMeter *GasMeter,
	store KVStore,
	api *GoAPI,
	querier *Querier,
	gasLimit uint64,
	sigInfo []byte,
	currentAdmin []byte,
	currentAdminProof []byte,
	newAdmin []byte,
) ([]byte, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	execIndex := recorder.NextExecutionIndex()

	logDebug("UpdateAdmin", "REPLAY mode: height=%d execIndex=%d", height, execIndex)
	if result, gas, err, found := replayExecution(store, gasMeter, execIndex); found {
		logDebug("UpdateAdmin", "REPLAY success: resultLen=%d gas=%d err=%v", len(result), gas, err)
		return result, err
	}
	logWarn("UpdateAdmin", "REPLAY FAILED: trace not found!")
	return nil, fmt.Errorf("UpdateAdmin replay failed: trace not found for height %d index %d", height, execIndex)
}

func Instantiate(
	cache Cache,
	code_id []byte,
	params []byte,
	msg []byte,
	gasMeter *GasMeter,
	store KVStore,
	api *GoAPI,
	querier *Querier,
	gasLimit uint64,
	sigInfo []byte,
	admin []byte,
) ([]byte, uint64, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	execIndex := recorder.NextExecutionIndex()

	logDebug("Instantiate", "REPLAY mode: height=%d execIndex=%d", height, execIndex)
	if result, gas, err, found := replayExecution(store, gasMeter, execIndex); found {
		logDebug("Instantiate", "REPLAY success: resultLen=%d gas=%d err=%v", len(result), gas, err)
		return result, gas, err
	}
	logWarn("Instantiate", "REPLAY FAILED: trace not found!")
	return nil, 0, fmt.Errorf("Instantiate replay failed: trace not found for height %d index %d", height, execIndex)
}

func Handle(
	cache Cache,
	code_id []byte,
	params []byte,
	msg []byte,
	gasMeter *GasMeter,
	store KVStore,
	api *GoAPI,
	querier *Querier,
	gasLimit uint64,
	sigInfo []byte,
	handleType types.HandleType,
) ([]byte, uint64, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	execIndex := recorder.NextExecutionIndex()

	logDebug("Handle", "REPLAY mode: height=%d execIndex=%d", height, execIndex)
	if result, gas, err, found := replayExecution(store, gasMeter, execIndex); found {
		logDebug("Handle", "REPLAY success: resultLen=%d gas=%d err=%v", len(result), gas, err)
		return result, gas, err
	}
	logWarn("Handle", "REPLAY FAILED: trace not found!")
	return nil, 0, fmt.Errorf("Handle replay failed: trace not found for height %d index %d", height, execIndex)
}

func Query(
	cache Cache,
	code_id []byte,
	params []byte,
	msg []byte,
	gasMeter *GasMeter,
	store KVStore,
	api *GoAPI,
	querier *Querier,
	gasLimit uint64,
) ([]byte, uint64, error) {
	return nil, 0, errors.New("Query not supported on non-SGX node")
}

func AnalyzeCode(
	cache Cache,
	codeHash []byte,
) (*v1types.AnalysisReport, error) {
	// Fetch the AnalyzeCode result from the SGX node via gRPC.
	// This is needed because non-SGX nodes don't have the enclave to analyze WASM bytecode,
	// but must know whether a contract has IBC entry points to register the correct IBC port.
	client := GetEcallClient()
	hasIBC, features, err := client.FetchAnalyzeCode(codeHash)
	if err != nil {
		logWarn("AnalyzeCode", "Failed to fetch from SGX node for code hash %s: %v, returning default (no IBC)", hex.EncodeToString(codeHash), err)
		return &v1types.AnalysisReport{
			HasIBCEntryPoints: false,
			RequiredFeatures:  "",
		}, nil
	}

	logDebug("AnalyzeCode", "Fetched AnalyzeCode for %s: hasIBC=%v features=%s", hex.EncodeToString(codeHash), hasIBC, features)
	return &v1types.AnalysisReport{
		HasIBCEntryPoints: hasIBC,
		RequiredFeatures:  features,
	}, nil
}

func KeyGen() ([]byte, error) {
	logInfo("KeyGen", "Skipped in replay mode, returning dummy key")
	return make([]byte, 32), nil
}

func CreateAttestationReport(no_epid bool, no_dcap bool, is_migration_report bool) (bool, error) {
	logInfo("CreateAttestationReport", "Skipped in replay mode")
	return true, nil
}

func GetNetworkPubkey(i_seed uint32) ([]byte, []byte) {
	return nil, nil
}

func GetEncryptedSeed(cert []byte) ([]byte, error) {
	recorder := GetRecorder()
	certHash := sha256.Sum256(cert)
	certHashHex := hex.EncodeToString(certHash[:])

	// Try local DB first
	if output, errMsg, found := recorder.ReplayGetEncryptedSeed(certHash[:]); found {
		if errMsg != "" {
			// Replay the exact same error the SGX enclave produced
			return nil, fmt.Errorf("%s", errMsg)
		}
		return output, nil
	}

	// Fetch from remote SGX node
	client := GetEcallClient()
	output, err := client.FetchEncryptedSeed(certHashHex)
	if err != nil {
		// Check if this is a FailedPrecondition error (recorded error from SGX node)
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			// The SGX node recorded the enclave error - cache and replay it
			enclaveErrMsg := st.Message()
			if cacheErr := recorder.RecordGetEncryptedSeedError(certHash[:], enclaveErrMsg); cacheErr != nil {
				logError("GetEncryptedSeed", "Failed to cache error: %v", cacheErr)
			}
			return nil, fmt.Errorf("%s", enclaveErrMsg)
		}
		return nil, fmt.Errorf("GetEncryptedSeed replay failed: %w", err)
	}

	// Cache locally
	if cacheErr := recorder.RecordGetEncryptedSeed(certHash[:], output); cacheErr != nil {
		logError("GetEncryptedSeed", "Failed to cache: %v", cacheErr)
	}
	return output, nil
}

func GetEncryptedGenesisSeed(cert []byte) ([]byte, error) {
	return nil, errors.New("GetEncryptedGenesisSeed not supported on non-SGX node")
}

func OnUpgradeProposalPassed(mrEnclaveHash []byte) error {
	return nil
}

func OnApproveMachineID(machineID []byte, proof *[32]byte, is_on_chain bool) error {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()

	// During node init (height=0), keeper loads stored proofs from state.
	// On SGX nodes this loads them into the enclave; on non-SGX there's
	// no enclave, so just skip — the proof already lives in the KV store.
	if height == 0 {
		logInfo("OnApproveMachineID", "Skipping at init (height=0, no enclave on non-SGX)")
		return nil
	}

	machineIDHex := fmt.Sprintf("%x", machineID)

	// Non-SGX nodes always fetch from the SGX node via gRPC
	client := GetEcallClient()
	maxRetries := 20
	retryDelay := 50 * time.Millisecond
	maxDelay := 2 * time.Second

	var proofData []byte
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		data, err := client.FetchMachineIDProof(height, machineIDHex)
		if err == nil && len(data) > 0 {
			proofData = data
			logInfo("OnApproveMachineID", "Fetched proof from SGX node: height=%d (attempt %d)", height, attempt+1)
			break
		}
		lastErr = err

		if attempt < maxRetries-1 {
			delay := retryDelay * time.Duration(1<<uint(attempt))
			if delay > maxDelay {
				delay = maxDelay
			}
			logDebug("OnApproveMachineID", "Waiting for SGX node proof: height=%d attempt=%d delay=%v", height, attempt+1, delay)
			time.Sleep(delay)
		}
	}

	if proofData == nil {
		logWarn("OnApproveMachineID", "No proof from SGX node after retries: height=%d, machineID=%s, lastErr=%v", height, machineIDHex, lastErr)
		return fmt.Errorf("no machine ID proof from SGX node for height %d: %v", height, lastErr)
	}

	copy(proof[:], proofData)
	return nil
}
