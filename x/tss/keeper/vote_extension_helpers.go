package keeper

import (
	"context"
)

// Vote Extension Helper Methods
// These methods generate TSS data for inclusion in vote extensions
// They are called by the ABCI vote extension handlers

// GenerateDKGRound1Data creates DKG Round 1 commitment data for this validator
// Returns serialized commitment bytes for inclusion in vote extension
func (k Keeper) GenerateDKGRound1Data(ctx context.Context, sessionID, validatorAddr string) []byte {
	return k.GenerateDKGRound1DataReal(ctx, sessionID, validatorAddr)
}

// GenerateDKGRound2Data creates DKG Round 2 share data for this validator
// Returns serialized share bytes for inclusion in vote extension
func (k Keeper) GenerateDKGRound2Data(ctx context.Context, sessionID, validatorAddr string) []byte {
	return k.GenerateDKGRound2DataReal(ctx, sessionID, validatorAddr)
}

// GenerateSigningCommitment creates signing Round 1 commitment for this validator
// Returns serialized commitment bytes for inclusion in vote extension
func (k Keeper) GenerateSigningCommitment(ctx context.Context, requestID, validatorAddr string) []byte {
	return k.GenerateSigningCommitmentReal(ctx, requestID, validatorAddr)
}

// GenerateSignatureShare creates signing Round 2 signature share for this validator
// Returns serialized share bytes for inclusion in vote extension
func (k Keeper) GenerateSignatureShare(ctx context.Context, requestID, validatorAddr string) []byte {
	return k.GenerateSignatureShareReal(ctx, requestID, validatorAddr)
}
