package keeper

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"

	"cosmossdk.io/core/store"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	ics23 "github.com/cosmos/ics23/go"
	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
	ra "github.com/scrtlabs/SecretNetwork/x/registration/remote_attestation"
)

// Keeper will have a reference to Wasmer with it's own data directory.
type Keeper struct {
	storeService store.KVStoreService
	cdc          codec.Codec
	enclave      EnclaveInterface
	router       baseapp.MessageRouter
	queryer      ABCIQueryer
	coldDataSet  bool
}

// NewKeeper creates a new contract Keeper instance
func NewKeeper(cdc codec.Codec, storeService store.KVStoreService, router baseapp.MessageRouter, enclave EnclaveInterface, homeDir string, bootstrap bool, q ABCIQueryer) Keeper {
	if !bootstrap {
		InitializeNode(homeDir, enclave)
	}

	return Keeper{
		storeService: storeService,
		cdc:          cdc,
		router:       router,
		enclave:      enclave,
		queryer:      q,
		coldDataSet:  false,
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

func InitializeNode(homeDir string, enclave EnclaveInterface) {
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
	_, err := enclave.LoadSeed(pk, sizedEndSeed)
	if err != nil {
		panic(errorsmod.Wrap(types.ErrSeedInitFailed, err.Error()))
	}
}

func (k Keeper) AddMachineSwapInfo(ctx sdk.Context, data []byte) error {
	store := k.storeService.OpenKVStore(ctx)

	var next_index [4]byte

	{
		bz, _ := store.Get(types.RegistrationMachineIndex)
		if bz != nil {
			next_index_numeric := binary.BigEndian.Uint32(bz) + 1
			binary.BigEndian.PutUint32(next_index[:], next_index_numeric)

		}
	}

	store.Set(types.RegistrationMachineIndex, next_index[:])

	key := make([]byte, 0, len(types.RegistrationMachinePrefix)+4)
	key = append(key, types.RegistrationMachinePrefix...)
	key = append(key, next_index[:]...)

	store.Set(key, data)

	return nil
}

func (k Keeper) OnNewMachine(ctx sdk.Context, id []byte) error {
	return k.AddMachineSwapInfo(ctx, id)
}

type ABCIQueryer interface {
	Query(ctx context.Context, req *abci.RequestQuery) (*abci.ResponseQuery, error)
}

// // Simple protobuf uvarint (VAR_PROTO) encoder
// func encodeUVarint(x uint64) []byte {
// 	buf := make([]byte, binary.MaxVarintLen64)
// 	n := binary.PutUvarint(buf, x)
// 	return buf[:n]
// }

// func LeafHashIAVL(prefix, key, value []byte) []byte {
// 	// Prehash key: NO_HASH
// 	k := key

// 	// Prehash value: SHA256
// 	vHash := sha256.Sum256(value)
// 	v := vHash[:]

// 	// Length encoding: VAR_PROTO (protobuf uvarint)
// 	// (for <128, it becomes one byte)
// 	kLen := encodeUVarint(uint64(len(k)))
// 	vLen := encodeUVarint(uint64(len(v)))

// 	// Build leaf bytes
// 	leafBytes := make([]byte, 0, len(prefix)+len(kLen)+len(k)+len(vLen)+len(v))
// 	leafBytes = append(leafBytes, prefix...)
// 	leafBytes = append(leafBytes, kLen...)
// 	leafBytes = append(leafBytes, k...)
// 	leafBytes = append(leafBytes, vLen...)
// 	leafBytes = append(leafBytes, v...)

// 	// Final leaf hash = SHA256(leafBytes)
// 	h := sha256.Sum256(leafBytes)
// 	return h[:]
// }

func SerializeMerkleProof(ops []cmtcrypto.ProofOp) (error, []byte) {
	proof_serialized := new(bytes.Buffer)

	_ = binary.Write(proof_serialized, binary.LittleEndian, uint32(len(ops)))

	// var hash []byte

	for _, op := range ops {
		// fmt.Printf("Op #%d:\n", i)
		// fmt.Printf("  Type: %s\n", op.Type)
		// fmt.Printf("  Key:  %s\n", hex.EncodeToString(op.Key))

		var cp ics23.CommitmentProof
		if err := proto.Unmarshal(op.Data, &cp); err != nil {
			return fmt.Errorf("failed to unmarshal ICS23 proof: %w", err), nil
		}

		switch p := cp.Proof.(type) {

		case *ics23.CommitmentProof_Exist:
			ep := p.Exist
			// fmt.Printf("ExistenceProof:\n")
			// fmt.Printf("  Key:   %s\n", hex.EncodeToString(ep.Key))
			// fmt.Printf("  Value: %s\n", hex.EncodeToString(ep.Value))

			lo := ep.Leaf
			// fmt.Printf("LeafOp:\n")
			// fmt.Printf(" hash        = %v\n", lo.Hash)
			// fmt.Printf(" prehash_key = %v\n", lo.PrehashKey)
			// fmt.Printf(" prehash_val = %v\n", lo.PrehashValue)
			// fmt.Printf(" length      = %v\n", lo.Length)
			// fmt.Printf(" prefix      = %X\n", lo.Prefix)

			// hash = LeafHashIAVL(lo.Prefix, ep.Key, ep.Value)
			// fmt.Println("Leaf hash: ", hex.EncodeToString(hash))

			_ = binary.Write(proof_serialized, binary.LittleEndian, uint32(len(lo.Prefix)))
			proof_serialized.Write(lo.Prefix)

			_ = binary.Write(proof_serialized, binary.LittleEndian, uint32(len(ep.Path)))

			for _, innerOp := range ep.Path {
				// fmt.Printf("    Prefix:   %s\n", hex.EncodeToString(innerOp.Prefix))
				// fmt.Printf("    Suffix:   %s\n", hex.EncodeToString(innerOp.Suffix))
				// fmt.Printf("    HashOp:   %d\n", innerOp.Hash)

				// hash_inp := append(innerOp.Prefix, hash...)
				// hash_inp = append(hash_inp, innerOp.Suffix...)
				// h := sha256.Sum256(hash_inp)
				// hash = h[:]

				_ = binary.Write(proof_serialized, binary.LittleEndian, uint32(len(innerOp.Prefix)))
				proof_serialized.Write(innerOp.Prefix)
				_ = binary.Write(proof_serialized, binary.LittleEndian, uint32(len(innerOp.Suffix)))
				proof_serialized.Write(innerOp.Suffix)

				// fmt.Printf("    value:   %s\n", hex.EncodeToString(hash))
			}

		default:
			return fmt.Errorf("unknown ICS23 proof type in CommitmentProof: %w", p), nil
		}
	}

	return nil, proof_serialized.Bytes()
}

func (k *Keeper) MaybeSetEnclaveColdData(ctx sdk.Context) error {
	if k.coldDataSet {
		return nil
	}

	k.coldDataSet = true

	// on-chain approved machine migrations

	store := k.storeService.OpenKVStore(ctx)
	prefixStore := prefix.NewStore(runtime.KVStoreAdapter(store), types.RegistrationMachinePrefix)
	it := prefixStore.Iterator(nil, nil)
	defer it.Close()

	// apphash := ctx.BlockHeader().AppHash
	// fmt.Println("**** AppHash: ", hex.EncodeToString(apphash))

	for ; it.Valid(); it.Next() {
		key := it.Key()
		value := it.Value()
		// fmt.Println("K: ", hex.EncodeToString(key))
		// fmt.Println("v: ", hex.EncodeToString(value))

		index := binary.BigEndian.Uint32(key)

		// fmt.Println("Querying the enclave cold evidence")

		height := ctx.BlockHeight() - 1

		key = append(types.RegistrationMachinePrefix, key...)
		req := &abci.RequestQuery{
			Path:   "/store/register/key", // <- your module storeKey
			Data:   key,                   // <- the actual key
			Height: height,
			Prove:  true,
		}

		resp, err := k.queryer.Query(ctx, req)

		var merkle_proof_serialized []byte = nil

		if err == nil {
			if resp != nil {
				// fmt.Println("Key: ", hex.EncodeToString(resp.Key))
				// fmt.Println("Value: ", hex.EncodeToString(resp.Value))
				err, merkle_proof_serialized = SerializeMerkleProof(resp.ProofOps.Ops)
			} else {
				fmt.Println("No Merkle proof for entry: ", hex.EncodeToString(key))
			}
		} else {
			fmt.Println("Merkle proof query error: ", err)
		}

		api.SubmitMachineSwap(index, value, merkle_proof_serialized)
	}

	return nil
}

func (k Keeper) RegisterNode(ctx sdk.Context, certificate ra.Certificate, replace_machine_id string) ([]byte, error) {
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

		// Note: don't skip envoking the enclave even if the node was already registered.
		// The enclave may realize the new node ownership

		var replaceMachineID [20]byte // this is by default

		{
			decoded, err := hex.DecodeString(replace_machine_id)
			if err == nil && len(decoded) == 20 {
				copy(replaceMachineID[:], decoded)
			}
		}

		var machineSwapInfo []byte
		encSeed, machineSwapInfo, err = k.enclave.GetEncryptedSeed(certificate, replaceMachineID[:])
		if err != nil {
			// return 0, errorsmod.Wrap(err, "cosmwasm create")
			return nil, errorsmod.Wrap(types.ErrAuthenticateFailed, err.Error())
		}

		if len(machineSwapInfo) == 104 {

			store_swap_info := make([]byte, 72)
			copy(store_swap_info[:52], machineSwapInfo[52:52+52])
			copy(store_swap_info[52:72], replaceMachineID[:])

			// last 20 bytes are the machine_id_pop - will be added later
			k.AddMachineSwapInfo(ctx, store_swap_info)
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
