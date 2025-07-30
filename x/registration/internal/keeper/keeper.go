package keeper

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"cosmossdk.io/core/store"
	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
	ra "github.com/scrtlabs/SecretNetwork/x/registration/remote_attestation"
)

// Keeper will have a reference to Wasmer with it's own data directory.
type Keeper struct {
	storeService store.KVStoreService
	cdc          codec.Codec
	enclave      EnclaveInterface
	router       baseapp.MessageRouter
}

// NewKeeper creates a new contract Keeper instance
func NewKeeper(cdc codec.Codec, storeService store.KVStoreService, router baseapp.MessageRouter, enclave EnclaveInterface, homeDir string, bootstrap bool) Keeper {
	if !bootstrap {
		InitializeNode(homeDir, enclave)
	}

	return Keeper{
		storeService: storeService,
		cdc:          cdc,
		router:       router,
		enclave:      enclave,
	}
}

func getSizedEncSeed(seed []byte) []byte {
	// Add size indicator infront of the seed
	// Size can always be represented by 1 byte as it can contain 2 seeds at most
	newEnc := make([]byte, len(seed)+1)
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, uint16(len(seed)))
	newEnc[0] = tmp[0]

	copy(newEnc[1:], seed)
	return newEnc
}

func getNewSeedParams(path string) ([]byte, []byte) {
	jsonContent, err := getFile(path)
	if err != nil {
		panic(errorsmod.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	var seedCfg types.SeedConfig
	err = json.Unmarshal(jsonContent, &seedCfg)
	if err != nil {
		panic(errorsmod.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	pk, enc, err := seedCfg.Decode()
	if err != nil {
		panic(errorsmod.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	return enc, pk
}

func getLegacySeedParams(path string) ([]byte, []byte) {
	jsonContent, err := getFile(path)
	if err != nil {
		panic(errorsmod.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	var seedCfg types.LegacySeedConfig
	err = json.Unmarshal(jsonContent, &seedCfg)
	if err != nil {
		panic(errorsmod.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	cert, enc, err := seedCfg.Decode()
	if err != nil {
		panic(errorsmod.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	pk, err := fetchPubKeyFromLegacyCert(cert)
	if err != nil {
		panic(errorsmod.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	return enc, pk
}

func createOldSecret(key []byte, seedFilePath string, enclave EnclaveInterface) error {
	seed, err := enclave.GetEncryptedGenesisSeed(key)
	if err != nil {
		return err
	}

	println(seed)

	cfg := types.LegacySeedConfig{
		EncryptedKey: fmt.Sprintf("%02x", seed),
		MasterCert:   types.LegacyIoMasterCertificate,
	}

	cfgBytes, err := json.Marshal(&cfg)
	if err != nil {
		return err
	}

	err = os.WriteFile(seedFilePath, cfgBytes, 0o600)
	if err != nil {
		return err
	}

	return nil
}

func InitializeNode(homeDir string, enclave EnclaveInterface) {
	apiKey, err := types.GetApiKey()
	if err != nil {
		panic(errorsmod.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	var (
		encSeed []byte
		pk      []byte
	)

	nodeDir := filepath.Join(homeDir, types.SecretNodeCfgFolder)

	// Read the most recent seed json config and pass the param to LoadSeed in order for him to understand wether new seed should be fetched from the service or not
	seedPath := filepath.Join(nodeDir, types.SecretNodeSeedNewConfig)
	legacySeedPath := filepath.Join(nodeDir, types.SecretNodeSeedLegacyConfig)

	if !fileExists(seedPath) {
		if fileExists(legacySeedPath) {
			encSeed, pk = getLegacySeedParams(legacySeedPath)
		}
	} else {
		encSeed, pk = getNewSeedParams(seedPath)
	}

	sizedEndSeed := getSizedEncSeed(encSeed)

	// On upgrade LoadSeed will write the new seed to "SeedPath -- seed.txt" which then will be parsed by the upgrade handler to create new_seed.json
	// On registration both seed.jsםn and new_seed.json will be created by 'secretd q register secret-network-params' on manual flow or by auto-registration flow"
	_, err = enclave.LoadSeed(pk, sizedEndSeed, apiKey)
	if err != nil {
		panic(errorsmod.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	if !fileExists(legacySeedPath) {
		sgxSecretsFolder := os.Getenv("SCRT_SGX_STORAGE")
		if sgxSecretsFolder == "" {
			sgxSecretsFolder = os.ExpandEnv("/opt/secret/.sgx_secrets")
		}

		sgxAttestationCertPath := filepath.Join(sgxSecretsFolder, types.AttestationCertPath)
		if !fileExists(sgxAttestationCertPath) {
			fmt.Printf("Failed to create legacy seed file. Attestation certificate does not exist in %s. Try to re-initialize the enclave\n", sgxAttestationCertPath)
			return
		}

		cert, err := os.ReadFile(sgxAttestationCertPath)
		if err != nil {
			panic(errorsmod.Wrap(types.ErrSeedInitFailed, fmt.Sprintf("Failed to read attestation certificate at %s", sgxAttestationCertPath)))
		}

		key, err := ra.UNSAFE_VerifyRaCert(cert)
		if err != nil {
			panic(errorsmod.Wrap(types.ErrSeedInitFailed, "Failed to get key from cert"))
		}

		err = createOldSecret(key, legacySeedPath, enclave)
		if err != nil {
			panic(errorsmod.Wrap(types.ErrSeedInitFailed, fmt.Sprintf("%s was not found and could not be created", legacySeedPath)))
		}
	}
}

func (k Keeper) RegisterNode(ctx sdk.Context, certificate ra.Certificate) ([]byte, error) {
	// fmt.Println("RegisterNode")
	var encSeed []byte
	var publicKey []byte

	if isSimulationMode(ctx) {
		// any sha256 hash is good enough
		encSeed = make([]byte, 32)
	} else {

		publicKey_, err := ra.VerifyCombinedCert(certificate)
		if err != nil {
			return nil, errorsmod.Wrap(types.ErrAuthenticateFailed, err.Error())
		}

		publicKey = publicKey_

		isAuth, err := k.isNodeAuthenticated(ctx, publicKey)
		if err != nil {
			return nil, errorsmod.Wrap(types.ErrAuthenticateFailed, err.Error())
		}
		if isAuth {
			return k.getRegistrationInfo(ctx, publicKey).EncryptedSeed, nil
		}

		encSeed, err = k.enclave.GetEncryptedSeed(certificate)
		if err != nil {
			// return 0, errorsmod.Wrap(err, "cosmwasm create")
			return nil, errorsmod.Wrap(types.ErrAuthenticateFailed, err.Error())
		}
	}

	regInfo := types.RegistrationNodeInfo{
		Certificate:   certificate,
		EncryptedSeed: encSeed,
	}

	var err error
	if isSimulationMode(ctx) {
		err = k.SetRegistrationInfo(ctx, regInfo)
	} else {
		err = k.SetRegistrationInfo_Verified(ctx, regInfo, publicKey)
	}

	if err != nil {
		ctx.Logger().Error("[-] Register node failed", "error", err.Error())
	} else {
		ctx.Logger().Info("[+] Register node success", "seed", hex.EncodeToString(encSeed))
	}

	return encSeed, nil
}

// returns true when simulation mode used by gas=auto queries
func isSimulationMode(ctx sdk.Context) bool {
	return ctx.GasMeter().Limit() == 0 && ctx.BlockHeight() != 0
}

func fetchPubKeyFromLegacyCert(cert []byte) ([]byte, error) {
	pk, err := FetchRawPubKeyFromLegacyCert(cert)
	if err != nil {
		return nil, err
	}

	return pk, nil
}

func FetchRawPubKeyFromLegacyCert(cert []byte) ([]byte, error) {
	pk, err := ra.VerifyRaCert(cert)
	if err != nil {
		return nil, err
	}

	return pk, nil
}

func IsHexString(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}
