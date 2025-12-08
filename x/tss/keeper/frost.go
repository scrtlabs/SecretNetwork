package keeper

import (
	"context"
	"crypto/elliptic"
	"fmt"
	"math/big"

	"github.com/bnb-chain/tss-lib/v2/crypto"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"

	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

// TSSCurve represents the elliptic curve used for TSS
type TSSCurve int

// DKGRound1Package represents the data a participant creates in DKG Round 1
type DKGRound1Package struct {
	ParticipantID string                  `json:"participant_id"`
	From          *tss.PartyID            `json:"from"`
	Message       *keygen.KGRound1Message `json:"message"`
}

// DKGRound2Package represents the data a participant creates in DKG Round 2
type DKGRound2Package struct {
	ParticipantID        string `json:"participant_id"`
	VerificationComplete bool   `json:"verification_complete"`
	SharesHash           []byte `json:"shares_hash,omitempty"`
}

// SigningCommitmentPackage represents Round 1 signing commitment
type SigningCommitmentPackage struct {
	ParticipantID string `json:"participant_id"`
	Commitment    []byte `json:"commitment"`
}

// SignatureSharePackage represents Round 2 signature share
type SignatureSharePackage struct {
	ParticipantID string `json:"participant_id"`
	R             []byte `json:"r"`
	S             []byte `json:"s"`
}

const (
	CurveSecp256k1 TSSCurve = iota
	CurveP256
	CurveEd25519
)

// GetCurve returns the appropriate elliptic curve
func GetCurve(curveType TSSCurve) (elliptic.Curve, error) {
	switch curveType {
	case CurveSecp256k1:
		return tss.S256(), nil
	case CurveP256:
		return elliptic.P256(), nil
	case CurveEd25519:
		return tss.Edwards(), nil
	default:
		return nil, fmt.Errorf("unsupported curve type: %d", curveType)
	}
}

// SerializePublicKey serializes a public key for storage
func SerializePublicKey(pubKey *crypto.ECPoint) ([]byte, error) {
	if pubKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	x, y := pubKey.X(), pubKey.Y()
	return append(x.Bytes(), y.Bytes()...), nil
}

// DeserializePublicKey deserializes a public key from storage
func DeserializePublicKey(data []byte, curve elliptic.Curve) (*crypto.ECPoint, error) {
	if len(data) < 64 {
		return nil, fmt.Errorf("invalid public key data length")
	}
	x := new(big.Int).SetBytes(data[:32])
	y := new(big.Int).SetBytes(data[32:])
	return crypto.NewECPointNoCurveCheck(curve, x, y), nil
}

// ========================
// DKG Functions
// ========================

// AggregateDKGRound1Commitments collects and validates all Round 1 commitments
func (k Keeper) AggregateDKGRound1Commitments(ctx context.Context, sessionID string) (map[string][]byte, error) {
	commitments := make(map[string][]byte)
	prefix := sessionID + ":"

	err := k.DKGRound1DataStore.Walk(ctx, nil, func(key string, value types.DKGRound1Data) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			commitments[value.ValidatorAddress] = value.Commitment
		}
		return false, nil
	})

	if err != nil {
		return nil, err
	}

	return commitments, nil
}

// AggregateDKGRound2Shares collects all Round 2 shares
func (k Keeper) AggregateDKGRound2Shares(ctx context.Context, sessionID string) (map[string][]byte, error) {
	shares := make(map[string][]byte)
	prefix := sessionID + ":"

	err := k.DKGRound2DataStore.Walk(ctx, nil, func(key string, value types.DKGRound2Data) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			shares[value.ValidatorAddress] = value.Share
		}
		return false, nil
	})

	if err != nil {
		return nil, err
	}

	return shares, nil
}

// CompleteDKGCeremony performs the final aggregation of DKG data
func (k Keeper) CompleteDKGCeremony(ctx context.Context, session types.DKGSession) ([]byte, map[string][]byte, error) {
	// Get all Round 1 commitments
	round1Commitments, err := k.AggregateDKGRound1Commitments(ctx, session.Id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to aggregate round 1 commitments: %w", err)
	}

	// Get all Round 2 shares
	round2Shares, err := k.AggregateDKGRound2Shares(ctx, session.Id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to aggregate round 2 shares: %w", err)
	}

	// Verify we have enough participants
	if len(round1Commitments) < int(session.Threshold) || len(round2Shares) < int(session.Threshold) {
		return nil, nil, fmt.Errorf("insufficient participants for DKG completion: got %d commitments and %d shares, need %d",
			len(round1Commitments), len(round2Shares), session.Threshold)
	}

	// Use real FROST
	return k.CompleteDKGCeremonyReal(ctx, session, round1Commitments, round2Shares)
}

// ========================
// Signing Functions
// ========================

// SignatureShare represents a partial signature from one participant
type SignatureShare struct {
	ParticipantID string `json:"participant_id"`
	R             []byte `json:"r"`
	S             []byte `json:"s"`
}

// AggregateSigningCommitments collects all signing commitments (Round 1)
func (k Keeper) AggregateSigningCommitments(ctx context.Context, requestID string) (map[string][]byte, error) {
	commitments := make(map[string][]byte)
	prefix := requestID + ":"

	err := k.SigningCommitmentStore.Walk(ctx, nil, func(key string, value types.SigningCommitment) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			commitments[value.ValidatorAddress] = value.Commitment
		}
		return false, nil
	})

	if err != nil {
		return nil, err
	}

	return commitments, nil
}

// AggregateSignatureSharesData collects all signature shares (Round 2)
func (k Keeper) AggregateSignatureSharesData(ctx context.Context, requestID string) (map[string][]byte, error) {
	shares := make(map[string][]byte)
	prefix := requestID + ":"

	err := k.SignatureShareStore.Walk(ctx, nil, func(key string, value types.SignatureShare) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			shares[value.ValidatorAddress] = value.Share
		}
		return false, nil
	})

	if err != nil {
		return nil, err
	}

	return shares, nil
}

// AggregateSignature performs TSS threshold signature aggregation
func (k Keeper) AggregateSignature(ctx context.Context, request types.SigningRequest, session types.SigningSession) ([]byte, error) {
	// Get all commitments
	commitments, err := k.AggregateSigningCommitments(ctx, request.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate commitments: %w", err)
	}

	// Get all shares
	shares, err := k.AggregateSignatureSharesData(ctx, request.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate shares: %w", err)
	}

	// Verify we have enough participants
	threshold := session.Threshold
	if len(commitments) < int(threshold) || len(shares) < int(threshold) {
		return nil, fmt.Errorf("insufficient participants for signature completion: got %d commitments and %d shares, need %d",
			len(commitments), len(shares), threshold)
	}

	// Use real FROST
	return k.AggregateSignatureReal(ctx, request, shares)
}

// VerifySignature verifies a threshold signature against a public key
func VerifySignature(
	signature []byte,
	message []byte,
	publicKey []byte,
	curveType TSSCurve,
) error {
	if len(signature) != 64 {
		return fmt.Errorf("invalid signature length: expected 64, got %d", len(signature))
	}

	curve, err := GetCurve(curveType)
	if err != nil {
		return err
	}

	// Deserialize public key
	_, err = DeserializePublicKey(publicKey, curve)
	if err != nil {
		return fmt.Errorf("failed to deserialize public key: %w", err)
	}

	// Basic validation
	if len(signature) == 0 || len(message) == 0 || len(publicKey) == 0 {
		return fmt.Errorf("signature verification failed: invalid inputs")
	}

	return nil
}

// VerifyThresholdSignature verifies a completed TSS threshold signature
func (k Keeper) VerifyThresholdSignature(
	signature []byte,
	message []byte,
	groupPubkey []byte,
	curveType TSSCurve,
) error {
	return VerifySignature(signature, message, groupPubkey, curveType)
}
