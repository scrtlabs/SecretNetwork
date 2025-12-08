package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/taurusgroup/frost-ed25519/pkg/eddsa"
	"github.com/taurusgroup/frost-ed25519/pkg/frost"
	"github.com/taurusgroup/frost-ed25519/pkg/frost/keygen"
	"github.com/taurusgroup/frost-ed25519/pkg/frost/party"
	"github.com/taurusgroup/frost-ed25519/pkg/frost/sign"
	"github.com/taurusgroup/frost-ed25519/pkg/helpers"
	"github.com/taurusgroup/frost-ed25519/pkg/state"

	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

// FROSTStateManager manages FROST protocol state for validators
// State is kept in memory since validators need it across blocks
type FROSTStateManager struct {
	mu sync.RWMutex

	// DKG state per session
	dkgStates  map[string]*state.State
	dkgOutputs map[string]*keygen.Output

	// Signing state per request
	signStates  map[string]*state.State
	signOutputs map[string]*sign.Output

	// Stored key shares for signing (indexed by keySetID)
	keyShares    map[string]*eddsa.SecretShare
	publicShares map[string]*eddsa.Public
}

// Global state manager (validators maintain this across blocks)
var frostStateManager = &FROSTStateManager{
	dkgStates:    make(map[string]*state.State),
	dkgOutputs:   make(map[string]*keygen.Output),
	signStates:   make(map[string]*state.State),
	signOutputs:  make(map[string]*sign.Output),
	keyShares:    make(map[string]*eddsa.SecretShare),
	publicShares: make(map[string]*eddsa.Public),
}

// ========================
// DKG Functions
// ========================

// InitDKGState initializes FROST DKG state for this validator
func (k Keeper) InitDKGState(sessionID string, selfIndex int, participantCount int, threshold uint32) error {
	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	// Check if already initialized
	if _, exists := frostStateManager.dkgStates[sessionID]; exists {
		return nil // Already initialized
	}

	// Create party IDs (1-indexed as FROST expects)
	partyIDs := make(party.IDSlice, participantCount)
	for i := 0; i < participantCount; i++ {
		partyIDs[i] = party.ID(i + 1)
	}

	// Our party ID (1-indexed)
	selfID := party.ID(selfIndex + 1)

	// Initialize FROST keygen state
	frostState, output, err := frost.NewKeygenState(selfID, partyIDs, party.Size(threshold), 0)
	if err != nil {
		return fmt.Errorf("failed to init DKG state: %w", err)
	}

	frostStateManager.dkgStates[sessionID] = frostState
	frostStateManager.dkgOutputs[sessionID] = output

	return nil
}

// GenerateDKGRound1Message generates real FROST DKG Round 1 data
func (k Keeper) GenerateDKGRound1Message(ctx context.Context, sessionID, validatorAddr string) ([]byte, error) {
	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	frostState, exists := frostStateManager.dkgStates[sessionID]
	if !exists {
		return nil, fmt.Errorf("DKG state not initialized for session %s", sessionID)
	}

	// Process round 1 (no input messages for first round)
	msgs, err := helpers.PartyRoutine(nil, frostState)
	if err != nil {
		return nil, fmt.Errorf("failed to generate round 1: %w", err)
	}

	// Combine all messages into a single package
	pkg := FROSTDKGRound1Msg{
		SessionID:    sessionID,
		ValidatorAddr: validatorAddr,
		Messages:     msgs,
	}

	return json.Marshal(pkg)
}

// ProcessDKGRound1Messages processes Round 1 messages and generates Round 2
func (k Keeper) ProcessDKGRound1Messages(sessionID string, round1Messages [][]byte) ([]byte, error) {
	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	frostState, exists := frostStateManager.dkgStates[sessionID]
	if !exists {
		return nil, fmt.Errorf("DKG state not initialized for session %s", sessionID)
	}

	// Collect all inner messages
	var allMsgs [][]byte
	for _, msgData := range round1Messages {
		var pkg FROSTDKGRound1Msg
		if err := json.Unmarshal(msgData, &pkg); err != nil {
			continue
		}
		allMsgs = append(allMsgs, pkg.Messages...)
	}

	// Process round 1 messages to generate round 2
	msgs, err := helpers.PartyRoutine(allMsgs, frostState)
	if err != nil {
		return nil, fmt.Errorf("failed to process round 1: %w", err)
	}

	// Package round 2 messages
	pkg := FROSTDKGRound2Msg{
		SessionID: sessionID,
		Messages:  msgs,
	}

	return json.Marshal(pkg)
}

// ProcessDKGRound2Messages processes Round 2 messages and finalizes DKG
func (k Keeper) ProcessDKGRound2Messages(sessionID string, round2Messages [][]byte) (*eddsa.PublicKey, *eddsa.SecretShare, *eddsa.Public, error) {
	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	frostState, exists := frostStateManager.dkgStates[sessionID]
	if !exists {
		return nil, nil, nil, fmt.Errorf("DKG state not initialized for session %s", sessionID)
	}

	output, exists := frostStateManager.dkgOutputs[sessionID]
	if !exists {
		return nil, nil, nil, fmt.Errorf("DKG output not initialized for session %s", sessionID)
	}

	// Collect all inner messages
	var allMsgs [][]byte
	for _, msgData := range round2Messages {
		var pkg FROSTDKGRound2Msg
		if err := json.Unmarshal(msgData, &pkg); err != nil {
			continue
		}
		allMsgs = append(allMsgs, pkg.Messages...)
	}

	// Process round 2 messages to finalize
	_, err := helpers.PartyRoutine(allMsgs, frostState)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to process round 2: %w", err)
	}

	// Wait for completion
	if err := frostState.WaitForError(); err != nil {
		return nil, nil, nil, fmt.Errorf("DKG failed: %w", err)
	}

	// Get results
	groupKey := output.Public.GroupKey
	secretShare := output.SecretKey
	publicShares := output.Public

	return groupKey, secretShare, publicShares, nil
}

// StoreFROSTKeyShareTemporary stores the FROST key share temporarily in memory
// Used during KEY_SUBMISSION phase before encryption and on-chain storage
// The key share will be cleared after encryption
func (k Keeper) StoreFROSTKeyShareTemporary(keySetID string, secretShare *eddsa.SecretShare, publicShares *eddsa.Public) {
	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	frostStateManager.keyShares[keySetID] = secretShare
	frostStateManager.publicShares[keySetID] = publicShares
}

// GetDKGGroupPubKey returns the group public key from the DKG output
func (k Keeper) GetDKGGroupPubKey(sessionID string) ([]byte, error) {
	frostStateManager.mu.RLock()
	defer frostStateManager.mu.RUnlock()

	output, exists := frostStateManager.dkgOutputs[sessionID]
	if !exists {
		return nil, fmt.Errorf("DKG output not found for session %s", sessionID)
	}

	if output.Public == nil || output.Public.GroupKey == nil {
		return nil, fmt.Errorf("group key not available for session %s", sessionID)
	}

	return output.Public.GroupKey.ToEd25519(), nil
}

// GetFROSTKeyShareForEncryption returns the FROST key shares for encryption
// Used during KEY_SUBMISSION phase
func (k Keeper) GetFROSTKeyShareForEncryption(keySetID string) (*eddsa.SecretShare, *eddsa.Public, error) {
	frostStateManager.mu.RLock()
	defer frostStateManager.mu.RUnlock()

	secretShare, exists := frostStateManager.keyShares[keySetID]
	if !exists {
		return nil, nil, fmt.Errorf("secret share not found for keyset %s", keySetID)
	}

	publicShares, exists := frostStateManager.publicShares[keySetID]
	if !exists {
		return nil, nil, fmt.Errorf("public shares not found for keyset %s", keySetID)
	}

	return secretShare, publicShares, nil
}

// ClearFROSTKeyShare removes the FROST key share from memory
// Called after the key share has been encrypted and stored on-chain
func (k Keeper) ClearFROSTKeyShare(keySetID string) {
	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	delete(frostStateManager.keyShares, keySetID)
	delete(frostStateManager.publicShares, keySetID)
}

// GenerateEncryptedKeySubmission generates an encrypted key share submission for on-chain storage
// Returns: encryptedSecretShare, encryptedPublicShares, ephemeralPubKey, error
func (k Keeper) GenerateEncryptedKeySubmission(ctx context.Context, sessionID, validatorAddr string) ([]byte, []byte, []byte, error) {
	// Get the DKG session to find the keyset ID
	session, err := k.GetDKGSession(ctx, sessionID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get DKG session: %w", err)
	}

	keySetID := session.KeySetId

	// Get the FROST key shares from memory (stored during Round 2 processing)
	secretShare, publicShares, err := k.GetFROSTKeyShareForEncryption(keySetID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get FROST key shares: %w", err)
	}

	// Get the validator's Ed25519 public key
	validatorPubKey, err := k.GetValidatorPubKeyByConsAddr(ctx, validatorAddr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get validator public key: %w", err)
	}

	// Serialize the secret share
	secretShareBytes, err := json.Marshal(secretShare)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to serialize secret share: %w", err)
	}

	// Serialize the public shares
	publicSharesBytes, err := json.Marshal(publicShares)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to serialize public shares: %w", err)
	}

	// Encrypt the secret share with the validator's public key
	encSecretShare, ephemeralPubKey1, err := EncryptKeyShareForChain(secretShareBytes, validatorPubKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to encrypt secret share: %w", err)
	}

	// Encrypt the public shares with the same ephemeral key for consistency
	// Note: We use a new ephemeral key for each piece for better security
	encPublicShares, _, err := EncryptKeyShareForChain(publicSharesBytes, validatorPubKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to encrypt public shares: %w", err)
	}

	// Clear the key share from memory after encryption
	// (it will be loaded from chain when needed for signing)
	k.ClearFROSTKeyShare(keySetID)

	return encSecretShare, encPublicShares, ephemeralPubKey1, nil
}

// LoadKeyShareFromChain loads and decrypts a key share from on-chain storage
// This is called on-demand when signing is needed
// The decrypted key is stored in memory temporarily and should be cleared after use
func (k Keeper) LoadKeyShareFromChain(ctx context.Context, keySetID string) error {
	// Get this validator's address
	validatorAddr, err := k.GetValidatorAddress(ctx)
	if err != nil {
		return fmt.Errorf("failed to get validator address: %w", err)
	}

	// Get the encrypted key share from chain
	keyShare, err := k.GetKeyShare(ctx, keySetID, validatorAddr)
	if err != nil {
		return fmt.Errorf("failed to get key share from chain: %w", err)
	}

	// Check if we have encrypted data
	if len(keyShare.EncryptedSecretShare) == 0 {
		return fmt.Errorf("no encrypted key share found on chain for keyset %s", keySetID)
	}

	// Get the validator's private key for decryption
	validatorPrivKey := k.GetValidatorPrivateKey()
	if len(validatorPrivKey) == 0 {
		return fmt.Errorf("validator private key not available for decryption")
	}

	// Decrypt the secret share
	secretShareBytes, err := DecryptKeyShareFromChain(
		keyShare.EncryptedSecretShare,
		keyShare.EphemeralPubkey,
		validatorPrivKey,
	)
	if err != nil {
		return fmt.Errorf("failed to decrypt secret share: %w", err)
	}

	// Decrypt the public shares
	publicSharesBytes, err := DecryptKeyShareFromChain(
		keyShare.EncryptedPublicShares,
		keyShare.EphemeralPubkey,
		validatorPrivKey,
	)
	if err != nil {
		return fmt.Errorf("failed to decrypt public shares: %w", err)
	}

	// Deserialize the secret share
	var secretShare eddsa.SecretShare
	if err := json.Unmarshal(secretShareBytes, &secretShare); err != nil {
		return fmt.Errorf("failed to deserialize secret share: %w", err)
	}

	// Deserialize the public shares
	var publicShares eddsa.Public
	if err := json.Unmarshal(publicSharesBytes, &publicShares); err != nil {
		return fmt.Errorf("failed to deserialize public shares: %w", err)
	}

	// Store temporarily in memory for signing
	frostStateManager.mu.Lock()
	frostStateManager.keyShares[keySetID] = &secretShare
	frostStateManager.publicShares[keySetID] = &publicShares
	frostStateManager.mu.Unlock()

	return nil
}

// ClearKeyShareAfterUse removes a key share from memory after signing is complete
// This ensures keys are only in memory during the signing operation
func (k Keeper) ClearKeyShareAfterUse(keySetID string) {
	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	delete(frostStateManager.keyShares, keySetID)
	delete(frostStateManager.publicShares, keySetID)
}

// CleanupDKGState removes DKG state after completion
func (k Keeper) CleanupDKGState(sessionID string) {
	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	delete(frostStateManager.dkgStates, sessionID)
	delete(frostStateManager.dkgOutputs, sessionID)
}

// ========================
// Signing Functions
// ========================

// InitSignState initializes FROST signing state for this validator
// This function loads key shares from chain on-demand if not already in memory
func (k Keeper) InitSignState(ctx context.Context, requestID, keySetID string, selfIndex int, signerIndices []int, message []byte) error {
	// First check (without lock) if already initialized
	frostStateManager.mu.RLock()
	_, alreadyInit := frostStateManager.signStates[requestID]
	frostStateManager.mu.RUnlock()
	if alreadyInit {
		return nil // Already initialized
	}

	// Check if key shares are in memory, if not load from chain
	frostStateManager.mu.RLock()
	_, hasKey := frostStateManager.keyShares[keySetID]
	frostStateManager.mu.RUnlock()

	if !hasKey {
		// Load key share from chain on-demand (decrypts using validator private key)
		if err := k.LoadKeyShareFromChain(ctx, keySetID); err != nil {
			return fmt.Errorf("failed to load key share from chain: %w", err)
		}
		fmt.Printf("Loaded and decrypted key share from chain for keyset: %s\n", keySetID)
	}

	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	// Double-check after acquiring lock
	if _, exists := frostStateManager.signStates[requestID]; exists {
		return nil // Already initialized
	}

	// Get stored key shares (now should be in memory)
	secretShare, exists := frostStateManager.keyShares[keySetID]
	if !exists {
		return fmt.Errorf("no key share found for keyset %s after loading", keySetID)
	}

	publicShares, exists := frostStateManager.publicShares[keySetID]
	if !exists {
		return fmt.Errorf("no public shares found for keyset %s after loading", keySetID)
	}

	// Create signer party IDs (1-indexed)
	signerIDs := make(party.IDSlice, len(signerIndices))
	for i, idx := range signerIndices {
		signerIDs[i] = party.ID(idx + 1)
	}

	// Initialize FROST sign state
	signState, signOutput, err := frost.NewSignState(signerIDs, secretShare, publicShares, message, 0)
	if err != nil {
		return fmt.Errorf("failed to init sign state: %w", err)
	}

	frostStateManager.signStates[requestID] = signState
	frostStateManager.signOutputs[requestID] = signOutput

	return nil
}

// GenerateSigningRound1Message generates real FROST signing commitment
func (k Keeper) GenerateSigningRound1Message(requestID, validatorAddr string) ([]byte, error) {
	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	signState, exists := frostStateManager.signStates[requestID]
	if !exists {
		return nil, fmt.Errorf("sign state not initialized for request %s", requestID)
	}

	// Process round 1 (no input for first round)
	msgs, err := helpers.PartyRoutine(nil, signState)
	if err != nil {
		return nil, fmt.Errorf("failed to generate signing round 1: %w", err)
	}

	pkg := FROSTSignRound1Msg{
		RequestID:    requestID,
		ValidatorAddr: validatorAddr,
		Messages:     msgs,
	}

	return json.Marshal(pkg)
}

// ProcessSigningRound1Messages processes commitments and generates signature shares
func (k Keeper) ProcessSigningRound1Messages(requestID string, round1Messages [][]byte) ([]byte, error) {
	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	signState, exists := frostStateManager.signStates[requestID]
	if !exists {
		return nil, fmt.Errorf("sign state not initialized for request %s", requestID)
	}

	// Collect all inner messages
	var allMsgs [][]byte
	for _, msgData := range round1Messages {
		var pkg FROSTSignRound1Msg
		if err := json.Unmarshal(msgData, &pkg); err != nil {
			continue
		}
		allMsgs = append(allMsgs, pkg.Messages...)
	}

	// Process round 1 messages to generate round 2 (signature shares)
	msgs, err := helpers.PartyRoutine(allMsgs, signState)
	if err != nil {
		return nil, fmt.Errorf("failed to process signing round 1: %w", err)
	}

	pkg := FROSTSignRound2Msg{
		RequestID: requestID,
		Messages:  msgs,
	}

	return json.Marshal(pkg)
}

// ProcessSigningRound2Messages processes signature shares and produces final signature
func (k Keeper) ProcessSigningRound2Messages(requestID string, round2Messages [][]byte) ([]byte, error) {
	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	signState, exists := frostStateManager.signStates[requestID]
	if !exists {
		return nil, fmt.Errorf("sign state not initialized for request %s", requestID)
	}

	signOutput, exists := frostStateManager.signOutputs[requestID]
	if !exists {
		return nil, fmt.Errorf("sign output not initialized for request %s", requestID)
	}

	// Collect all inner messages
	var allMsgs [][]byte
	for _, msgData := range round2Messages {
		var pkg FROSTSignRound2Msg
		if err := json.Unmarshal(msgData, &pkg); err != nil {
			continue
		}
		allMsgs = append(allMsgs, pkg.Messages...)
	}

	// Process round 2 to finalize signature
	_, err := helpers.PartyRoutine(allMsgs, signState)
	if err != nil {
		return nil, fmt.Errorf("failed to process signing round 2: %w", err)
	}

	// Wait for completion
	if err := signState.WaitForError(); err != nil {
		return nil, fmt.Errorf("signing failed: %w", err)
	}

	// Get signature
	sig := signOutput.Signature
	if sig == nil {
		return nil, fmt.Errorf("signature is nil")
	}

	// Marshal to bytes (Ed25519 format - 64 bytes)
	sigBytes := sig.ToEd25519()

	return sigBytes, nil
}

// CleanupSignState removes signing state after completion
func (k Keeper) CleanupSignState(requestID string) {
	frostStateManager.mu.Lock()
	defer frostStateManager.mu.Unlock()

	delete(frostStateManager.signStates, requestID)
	delete(frostStateManager.signOutputs, requestID)
}

// ========================
// Message Types
// ========================

// FROSTDKGRound1Msg wraps DKG Round 1 messages
type FROSTDKGRound1Msg struct {
	SessionID     string   `json:"session_id"`
	ValidatorAddr string   `json:"validator_addr"`
	Messages      [][]byte `json:"messages"`
}

// FROSTDKGRound2Msg wraps DKG Round 2 messages
type FROSTDKGRound2Msg struct {
	SessionID string   `json:"session_id"`
	Messages  [][]byte `json:"messages"`
}

// FROSTSignRound1Msg wraps signing Round 1 messages
type FROSTSignRound1Msg struct {
	RequestID     string   `json:"request_id"`
	ValidatorAddr string   `json:"validator_addr"`
	Messages      [][]byte `json:"messages"`
}

// FROSTSignRound2Msg wraps signing Round 2 messages
type FROSTSignRound2Msg struct {
	RequestID string   `json:"request_id"`
	Messages  [][]byte `json:"messages"`
}

// ========================
// Integration with existing code
// ========================

// CompleteDKGCeremonyReal performs DKG using real FROST
func (k Keeper) CompleteDKGCeremonyReal(ctx context.Context, session types.DKGSession, round1Data, round2Data map[string][]byte) ([]byte, map[string][]byte, error) {
	sessionID := session.Id

	// Collect round 1 messages
	var round1Messages [][]byte
	for _, data := range round1Data {
		round1Messages = append(round1Messages, data)
	}

	// Collect round 2 messages
	var round2Messages [][]byte
	for _, data := range round2Data {
		round2Messages = append(round2Messages, data)
	}

	// Finalize DKG
	groupKey, secretShare, publicShares, err := k.ProcessDKGRound2Messages(sessionID, round2Messages)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to complete DKG: %w", err)
	}

	// Store key shares temporarily for KEY_SUBMISSION phase encryption
	// These will be encrypted and stored on-chain, then cleared from memory
	k.StoreFROSTKeyShareTemporary(session.KeySetId, secretShare, publicShares)

	// Serialize group public key
	groupPubkeyBytes := groupKey.ToEd25519()

	// Create key share references for participants
	keyShares := make(map[string][]byte)
	for _, validatorAddr := range session.Participants {
		shareRef := map[string]interface{}{
			"keyset_id": session.KeySetId,
			"threshold": session.Threshold,
			"curve":     "ed25519",
			"protocol":  "frost",
		}
		shareRefBytes, _ := json.Marshal(shareRef)
		keyShares[validatorAddr] = shareRefBytes
	}

	// Cleanup DKG state
	k.CleanupDKGState(sessionID)

	return groupPubkeyBytes, keyShares, nil
}

// AggregateSignatureReal performs signature aggregation using real FROST
func (k Keeper) AggregateSignatureReal(ctx context.Context, request types.SigningRequest, round2Data map[string][]byte) ([]byte, error) {
	requestID := request.Id
	keySetID := request.KeySetId

	// Collect round 2 messages
	var round2Messages [][]byte
	for _, data := range round2Data {
		round2Messages = append(round2Messages, data)
	}

	// Finalize signature
	signature, err := k.ProcessSigningRound2Messages(requestID, round2Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate signature: %w", err)
	}

	// Cleanup sign state
	k.CleanupSignState(requestID)

	// Clear the key share from memory after signing is complete
	// Keys will be reloaded from chain on-demand for the next signing request
	k.ClearKeyShareAfterUse(keySetID)
	fmt.Printf("Cleared key share from memory after signing: %s\n", keySetID)

	return signature, nil
}
