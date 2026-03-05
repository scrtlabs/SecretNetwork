//go:build !secretcli && nosgx

package api

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	v1types "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v1"
)

type Cache struct{}

func HealthCheck() ([]byte, error) {
	return []byte("replay"), nil
}

func InitBootstrap(spid []byte, apiKey []byte) ([]byte, error) {
	return nil, nil
}

func SubmitBlockSignatures(header []byte, commit []byte, txs []byte, encRandom []byte, cronMsgs []byte) ([]byte, []byte, error) {
	// In non-SGX mode, retrieve the stream for SubmitBlockSignatures (recorded at index 0)
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()

	streamBytes, found := recorder.GetStreamFromMemory(0)
	if !found {
		return nil, nil, fmt.Errorf("SubmitBlockSignatures replay failed: stream not found for height %d", height)
	}

	return ReplayStreamForBlockSignatures(streamBytes)
}

func SubmitValidatorSetEvidence(evidence []byte) error {
	return errors.New("submit validator set evidence not supported on non-SGX node")
}

func LoadSeedToEnclave(masterKey []byte, seed []byte, apiKey []byte) (bool, error) {
	return true, nil
}

type Querier = types.Querier

func MigrationOp(op uint32) (bool, error) {
	return false, errors.New("MigrationOp not supported on non-SGX node")
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
	hash := sha256.Sum256(wasm)
	return hash[:], nil
}

func GetCode(cache Cache, code_id []byte) ([]byte, error) {
	return nil, errors.New("GetCode is not supported on non-SGX node")
}

// replayFromStream is a helper to fetch a stream from memory and replay it
func replayFromStream(funcName string, store KVStore, querier *Querier) ([]byte, uint64, uint64, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	execIndex := recorder.NextExecutionIndex()

	logDebug(funcName, "REPLAY mode: height=%d execIndex=%d", height, execIndex)
	streamBytes, found := recorder.GetStreamFromMemory(execIndex)
	if !found {
		logWarn(funcName, "REPLAY FAILED: stream not found!")
		return nil, 0, 0, fmt.Errorf("%s replay failed: stream not found for height %d index %d", funcName, height, execIndex)
	}
	return ReplayStream(store, querier, streamBytes)
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
) ([]byte, uint64, uint64, error) {
	return replayFromStream("Migrate", store, querier)
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
	result, _, _, err := replayFromStream("UpdateAdmin", store, querier)
	return result, err
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
) ([]byte, uint64, uint64, error) {
	return replayFromStream("Instantiate", store, querier)
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
) ([]byte, uint64, uint64, error) {
	return replayFromStream("Handle", store, querier)
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
	return nil, errors.New("AnalyzeCode not supported on non-SGX node")
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
	return nil, errors.New("GetEncryptedSeed not supported on non-SGX node")
}

func GetEncryptedGenesisSeed(cert []byte) ([]byte, error) {
	return nil, errors.New("GetEncryptedGenesisSeed not supported on non-SGX node")
}

func OnUpgradeProposalPassed(mrEnclaveHash []byte) error {
	return nil
}

func OnApproveMachineID(machineID []byte, proof *[32]byte, is_on_chain bool) error {
	return nil
}
