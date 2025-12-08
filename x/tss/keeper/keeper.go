package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	stakingKeeper *stakingkeeper.Keeper
	wasmKeeper    types.WasmKeeper

	// ValidatorConsensusAddress is this node's validator consensus address (hex format)
	// Set at startup from the priv_validator_key.json
	ValidatorConsensusAddress string

	// ValidatorPrivateKey is this node's validator Ed25519 private key (for decrypting key shares from chain)
	// Set at startup from the priv_validator_key.json
	// This is the raw 64-byte Ed25519 private key
	ValidatorPrivateKey []byte

	Schema collections.Schema
	Params collections.Item[types.Params]

	// KeySet and KeyShare stores (from x/mpc)
	// KeySetStore stores all KeySets by key_set_id
	KeySetStore collections.Map[string, types.KeySet]

	// KeyShareStore stores validator key shares per KeySet
	// Key: (key_set_id, validator_address)
	KeyShareStore collections.Map[collections.Pair[string, string], types.KeyShare]

	// DKG stores (from x/mpc)
	// DKGSessionStore stores active DKG sessions
	DKGSessionStore collections.Map[string, types.DKGSession]

	// DKGRound1DataStore stores Round 1 commitments
	// Key: "session_id:validator_address"
	DKGRound1DataStore collections.Map[string, types.DKGRound1Data]

	// DKGRound2DataStore stores Round 2 shares
	// Key: "session_id:validator_address"
	DKGRound2DataStore collections.Map[string, types.DKGRound2Data]

	// DKGKeySubmissionStore stores encrypted key share submissions
	// Key: "session_id:validator_address"
	DKGKeySubmissionStore collections.Map[string, types.DKGKeySubmission]

	// Signing stores (from x/signing)
	// SigningRequestStore stores signing requests by request_id
	SigningRequestStore collections.Map[string, types.SigningRequest]

	// SigningSessionStore stores active signing sessions by request_id
	SigningSessionStore collections.Map[string, types.SigningSession]

	// SigningCommitmentStore stores Round 1 commitments
	// Key: "request_id:validator_address"
	SigningCommitmentStore collections.Map[string, types.SigningCommitment]

	// SignatureShareStore stores Round 2 shares
	// Key: "request_id:validator_address"
	SignatureShareStore collections.Map[string, types.SignatureShare]
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	stakingKeeper *stakingkeeper.Keeper,
) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		storeService:  storeService,
		cdc:           cdc,
		addressCodec:  addressCodec,
		authority:     authority,
		stakingKeeper: stakingKeeper,

		Params: collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),

		// KeySet and DKG stores
		KeySetStore:        collections.NewMap(sb, types.KeySetPrefix, "keysets", collections.StringKey, codec.CollValue[types.KeySet](cdc)),
		KeyShareStore:      collections.NewMap(sb, types.KeySharePrefix, "keyshares", collections.PairKeyCodec(collections.StringKey, collections.StringKey), codec.CollValue[types.KeyShare](cdc)),
		DKGSessionStore:    collections.NewMap(sb, types.DKGSessionPrefix, "dkg_sessions", collections.StringKey, codec.CollValue[types.DKGSession](cdc)),
		DKGRound1DataStore: collections.NewMap(sb, types.DKGRound1DataPrefix, "dkg_round1_data", collections.StringKey, codec.CollValue[types.DKGRound1Data](cdc)),
		DKGRound2DataStore:    collections.NewMap(sb, types.DKGRound2DataPrefix, "dkg_round2_data", collections.StringKey, codec.CollValue[types.DKGRound2Data](cdc)),
		DKGKeySubmissionStore: collections.NewMap(sb, types.DKGKeySubmissionPrefix, "dkg_key_submissions", collections.StringKey, codec.CollValue[types.DKGKeySubmission](cdc)),

		// Signing stores
		SigningRequestStore:    collections.NewMap(sb, types.SigningRequestPrefix, "signing_requests", collections.StringKey, codec.CollValue[types.SigningRequest](cdc)),
		SigningSessionStore:    collections.NewMap(sb, types.SigningSessionPrefix, "signing_sessions", collections.StringKey, codec.CollValue[types.SigningSession](cdc)),
		SigningCommitmentStore: collections.NewMap(sb, types.SigningCommitmentPrefix, "signing_commitments", collections.StringKey, codec.CollValue[types.SigningCommitment](cdc)),
		SignatureShareStore:    collections.NewMap(sb, types.SignatureSharePrefix, "signature_shares", collections.StringKey, codec.CollValue[types.SignatureShare](cdc)),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() []byte {
	return k.authority
}

// SetValidatorConsensusAddress sets this node's validator consensus address
// Should be called at app startup with the address from priv_validator_key.json
func (k *Keeper) SetValidatorConsensusAddress(addr string) {
	k.ValidatorConsensusAddress = addr
}

// SetValidatorPrivateKey sets this node's validator Ed25519 private key
// Should be called at app startup with the key from priv_validator_key.json
// The key is used to decrypt key shares from on-chain storage
func (k *Keeper) SetValidatorPrivateKey(privKey []byte) {
	k.ValidatorPrivateKey = privKey
}

// GetValidatorPrivateKey returns this node's validator Ed25519 private key
// Used for decrypting key shares from on-chain storage
func (k Keeper) GetValidatorPrivateKey() []byte {
	return k.ValidatorPrivateKey
}

// SetWasmKeeper sets the wasm keeper for contract callbacks
// This is called after wasm keeper initialization due to initialization order
func (k *Keeper) SetWasmKeeper(wasmKeeper types.WasmKeeper) {
	k.wasmKeeper = wasmKeeper
}

// GetActiveValidatorAddresses returns all active validator consensus addresses
// Uses staking module to get validators
func (k Keeper) GetActiveValidatorAddresses(ctx context.Context) ([]string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get all validators from staking module
	validators, err := k.stakingKeeper.GetAllValidators(ctx)
	if err != nil {
		sdkCtx.Logger().Error("Failed to get all validators", "error", err)
		return nil, fmt.Errorf("failed to get validators: %w", err)
	}

	addresses := make([]string, 0)
	for _, val := range validators {
		// Only include bonded, non-jailed validators
		if !val.IsBonded() || val.IsJailed() {
			continue
		}

		// Get consensus public key
		consPubKey, err := val.ConsPubKey()
		if err != nil {
			continue
		}

		// Convert to consensus address (hex format)
		consAddr := sdk.ConsAddress(consPubKey.Address())
		hexAddr := fmt.Sprintf("%x", consAddr.Bytes())
		addresses = append(addresses, hexAddr)
	}

	return addresses, nil
}

// GetValidatorAddress returns this validator's consensus address
// Returns the address set at startup from priv_validator_key.json
func (k Keeper) GetValidatorAddress(ctx context.Context) (string, error) {
	return k.ValidatorConsensusAddress, nil
}

// GetValidatorPubKeyByConsAddr returns the Ed25519 public key for a validator by consensus address (hex)
func (k Keeper) GetValidatorPubKeyByConsAddr(ctx context.Context, consAddrHex string) ([]byte, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get all validators and find the one matching the consensus address
	validators, err := k.stakingKeeper.GetAllValidators(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get validators: %w", err)
	}

	for _, val := range validators {
		consPubKey, err := val.ConsPubKey()
		if err != nil {
			continue
		}

		// Convert to consensus address (hex format)
		consAddr := sdk.ConsAddress(consPubKey.Address())
		hexAddr := fmt.Sprintf("%x", consAddr.Bytes())

		if hexAddr == consAddrHex {
			// Return the raw Ed25519 public key bytes
			return consPubKey.Bytes(), nil
		}
	}

	sdkCtx.Logger().Error("Validator not found for consensus address", "consAddrHex", consAddrHex)
	return nil, fmt.Errorf("validator not found for consensus address: %s", consAddrHex)
}
