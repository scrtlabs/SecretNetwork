package types

import "cosmossdk.io/collections"

const (
	// ModuleName defines the module name
	ModuleName = "tss"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// GovModuleName duplicates the gov module's name to avoid a dependency with x/gov.
	// It should be synced with the gov module's name if it is ever changed.
	// See: https://github.com/cosmos/cosmos-sdk/blob/v0.52.0-beta.2/x/gov/types/keys.go#L9
	GovModuleName = "gov"
)

// ParamsKey is the prefix to retrieve all Params
var ParamsKey = collections.NewPrefix("p_tss")

// KeySet and KeyShare prefixes (from x/mpc)
// KeySetPrefix is the prefix for KeySet storage
var KeySetPrefix = collections.NewPrefix("keyset")

// KeySharePrefix is the prefix for KeyShare storage (per KeySet + validator)
var KeySharePrefix = collections.NewPrefix("keyshare")

// DKG prefixes (from x/mpc)
// DKGSessionPrefix is the prefix for DKGSession storage
var DKGSessionPrefix = collections.NewPrefix("dkg_session")

// DKGRound1DataPrefix is the prefix for DKG Round 1 commitment data
var DKGRound1DataPrefix = collections.NewPrefix("dkg_round1")

// DKGRound2DataPrefix is the prefix for DKG Round 2 share data
var DKGRound2DataPrefix = collections.NewPrefix("dkg_round2")

// DKGKeySubmissionPrefix is the prefix for encrypted key share submissions
var DKGKeySubmissionPrefix = collections.NewPrefix("dkg_key_submission")

// Signing prefixes (from x/signing)
// SigningRequestPrefix is the prefix for SigningRequest storage
var SigningRequestPrefix = collections.NewPrefix("signing_request")

// SigningSessionPrefix is the prefix for SigningSession storage
var SigningSessionPrefix = collections.NewPrefix("signing_session")

// SigningCommitmentPrefix is the prefix for SigningCommitment storage
var SigningCommitmentPrefix = collections.NewPrefix("signing_commitment")

// SignatureSharePrefix is the prefix for SignatureShare storage
var SignatureSharePrefix = collections.NewPrefix("signature_share")
