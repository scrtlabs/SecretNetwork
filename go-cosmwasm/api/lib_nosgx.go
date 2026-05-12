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

func InitBootstrap() ([]byte, error) {
	return nil, nil
}

func SubmitBlockSignatures(header []byte, commit []byte, txs []byte, encRandom []byte) ([]byte, []byte, error) {
	return nil, nil, errors.New("submit block signatures not supported on non-SGX node")
}

func SubmitValidatorSetEvidence(evidence []byte) error {
	logInfo("SubmitValidatorSetEvidence", "Skipped in replay mode")
	return nil
}

func LoadSeedToEnclave(masterKey []byte, seed []byte) (bool, error) {
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
		retryDelay := 2 * time.Second
		attempt := 0

		for {
			if client != nil && client.IsConnected() {
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
			}

			attempt++
			if attempt%15 == 1 { // Log every ~30 seconds
				logWarn("Create", "Waiting for SGX node Create result: height=%d wasmHash=%x attempt=%d", height, wasmHash[:8], attempt)
			}
			time.Sleep(retryDelay)
		}
	}

	// Non-replay mode (e.g. secretcli): no enclave available, use sha256 of wasm
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
	// This flag directly affects state (IBC port creation), so we MUST retry and fail loudly.
	client := GetEcallClient()
	maxRetries := 20
	retryDelay := 50 * time.Millisecond
	maxDelay := 2 * time.Second
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		hasIBC, features, err := client.FetchAnalyzeCode(codeHash)
		if err == nil {
			logDebug("AnalyzeCode", "Fetched AnalyzeCode for %s: hasIBC=%v features=%s", hex.EncodeToString(codeHash), hasIBC, features)
			return &v1types.AnalysisReport{
				HasIBCEntryPoints: hasIBC,
				RequiredFeatures:  features,
			}, nil
		}
		lastErr = err
		if attempt < maxRetries-1 {
			delay := retryDelay * time.Duration(1<<uint(attempt))
			if delay > maxDelay {
				delay = maxDelay
			}
			time.Sleep(delay)
		}
	}
	return nil, fmt.Errorf("AnalyzeCode: failed after %d retries for code hash %s: %v", maxRetries, hex.EncodeToString(codeHash), lastErr)
}

func KeyGen() ([]byte, error) {
	logInfo("KeyGen", "Skipped in replay mode, returning dummy key")
	return make([]byte, 32), nil
}

func CreateAttestationReport(is_migration_report bool) (bool, error) {
	logInfo("CreateAttestationReport", "Skipped in replay mode")
	return true, nil
}

func GetNetworkPubkey(i_seed uint32) ([]byte, []byte) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()

	nodePk, ioPk, err := GetEcallClient().FetchNetworkPubkey(height, i_seed)
	if err != nil {
		logError("GetNetworkPubkey", "Failed to fetch on replay: %v", err)
		return nil, nil
	}
	return nodePk, ioPk
}

func GetEncryptedSeed(cert []byte) ([]byte, error) {
	recorder := GetRecorder()
	certHash := sha256.Sum256(cert)
	certHashHex := hex.EncodeToString(certHash[:])

	logInfo("GetEncryptedSeed", "NON-SGX called: certHashHex=%s certLen=%d dbInitialized=%v",
		certHashHex, len(cert), recorder != nil && recorder.db != nil)

	height := recorder.GetCurrentBlockHeight()

	// Try local DB first
	if output, errMsg, found := recorder.ReplayGetEncryptedSeed(height, certHash[:]); found {
		if errMsg != "" {
			logInfo("GetEncryptedSeed", "Found CACHED ERROR in local DB for %s: %s", certHashHex, errMsg)
			return nil, fmt.Errorf("%s", errMsg)
		}
		logInfo("GetEncryptedSeed", "Found CACHED SUCCESS in local DB for %s (%d bytes)", certHashHex, len(output))
		return output, nil
	}
	logInfo("GetEncryptedSeed", "NOT in local DB for %s, will fetch from SGX node via gRPC", certHashHex)

	// Fetch from remote SGX node with retries (the SGX node may still be
	// processing the same block and recording the seed when we query)
	client := GetEcallClient()
	maxRetries := 20
	retryDelay := 50 * time.Millisecond
	maxDelay := 2 * time.Second

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		output, err := client.FetchEncryptedSeed(height, certHashHex)
		if err == nil {
			logInfo("GetEncryptedSeed", "Fetched seed from SGX node (attempt %d) for %s (%d bytes)",
				attempt+1, certHashHex, len(output))
			// Cache locally
			if cacheErr := recorder.RecordGetEncryptedSeed(height, certHash[:], output); cacheErr != nil {
				logError("GetEncryptedSeed", "Failed to cache: %v", cacheErr)
			}
			return output, nil
		}

		// Extract gRPC status for logging
		grpcCode := "unknown"
		grpcMsg := err.Error()
		if st, ok := status.FromError(err); ok {
			grpcCode = st.Code().String()
			grpcMsg = st.Message()
		}

		logInfo("GetEncryptedSeed", "gRPC attempt %d/%d for %s: code=%s msg=%s",
			attempt+1, maxRetries, certHashHex, grpcCode, grpcMsg)

		// Check if this is a FailedPrecondition error (recorded error from SGX node)
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			// The SGX node recorded the enclave error - cache and replay it
			enclaveErrMsg := st.Message()
			logInfo("GetEncryptedSeed", "SGX node returned FailedPrecondition (enclave error) for %s: %s",
				certHashHex, enclaveErrMsg)
			if cacheErr := recorder.RecordGetEncryptedSeedError(height, certHash[:], enclaveErrMsg); cacheErr != nil {
				logError("GetEncryptedSeed", "Failed to cache error: %v", cacheErr)
			}
			return nil, fmt.Errorf("%s", enclaveErrMsg)
		}

		lastErr = err

		// For NotFound, the SGX node may not have recorded yet — retry
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			if attempt < maxRetries-1 {
				time.Sleep(retryDelay)
				retryDelay *= 2
				if retryDelay > maxDelay {
					retryDelay = maxDelay
				}
			}
			continue
		}

		// For other errors (connection issues etc.), also retry
		if attempt < maxRetries-1 {
			time.Sleep(retryDelay)
			retryDelay *= 2
			if retryDelay > maxDelay {
				retryDelay = maxDelay
			}
		}
	}

	logError("GetEncryptedSeed", "EXHAUSTED all %d retries for %s. lastErr: %v", maxRetries, certHashHex, lastErr)
	return nil, fmt.Errorf("GetEncryptedSeed: failed after %d retries for cert hash %s: %v", maxRetries, certHashHex, lastErr)
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

	machineIDHex := fmt.Sprintf("%x", machineID)

	// During node init (height=0), keeper loads stored proofs from state.
	// On SGX nodes this loads them into the enclave; on non-SGX there's
	// no enclave, so just skip — the proof already lives in the KV store.
	if height == 0 {
		logInfo("OnApproveMachineID", "Skipping at init (height=0, no enclave on non-SGX)")
		return nil
	}

	// Non-SGX nodes always fetch from the SGX node via gRPC
	client := GetEcallClient()
	retryDelay := 2 * time.Second
	attempt := 0

	for {
		data, err := client.FetchMachineIDProof(height, machineIDHex)
		if err == nil && len(data) > 0 {
			logInfo("OnApproveMachineID", "Fetched proof from SGX node: height=%d (attempt %d)", height, attempt+1)
			copy(proof[:], data)
			return nil
		}

		attempt++
		if attempt%15 == 1 { // Log every ~30 seconds
			logWarn("OnApproveMachineID", "Waiting for SGX node proof: height=%d machineID=%s attempt=%d err=%v", height, machineIDHex, attempt, err)
		}
		time.Sleep(retryDelay)
	}
}
