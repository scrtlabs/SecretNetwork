package keeper

import (
	"context"
	"fmt"
)

// Vote Extension Helper Methods for Real FROST
// These generate real cryptographic data for vote extensions

// GenerateDKGRound1DataReal creates real FROST DKG Round 1 data
func (k Keeper) GenerateDKGRound1DataReal(ctx context.Context, sessionID, validatorAddr string) []byte {
	// Get the session
	session, err := k.GetDKGSession(ctx, sessionID)
	if err != nil {
		fmt.Printf("FROST DKG Round1: failed to get session: %v\n", err)
		return nil
	}

	// Find our participant index
	participantIndex := -1
	for i, addr := range session.Participants {
		if addr == validatorAddr {
			participantIndex = i
			break
		}
	}
	if participantIndex < 0 {
		fmt.Printf("FROST DKG Round1: validator %s not in participants\n", validatorAddr)
		return nil
	}

	// Initialize FROST DKG state if not already done
	if err := k.InitDKGState(sessionID, participantIndex, len(session.Participants), session.Threshold); err != nil {
		fmt.Printf("FROST DKG Round1: failed to init state: %v\n", err)
		return nil
	}

	// Generate Round 1 message
	msg, err := k.GenerateDKGRound1Message(ctx, sessionID, validatorAddr)
	if err != nil {
		fmt.Printf("FROST DKG Round1: failed to generate message: %v\n", err)
		return nil
	}

	return msg
}

// GenerateDKGRound2DataReal creates real FROST DKG Round 2 data
func (k Keeper) GenerateDKGRound2DataReal(ctx context.Context, sessionID, validatorAddr string) []byte {
	// Get all Round 1 data for this session
	round1Data, err := k.AggregateDKGRound1Commitments(ctx, sessionID)
	if err != nil {
		fmt.Printf("FROST DKG Round2: failed to get round 1 data: %v\n", err)
		return nil
	}

	// Collect round 1 messages
	var round1Messages [][]byte
	for _, data := range round1Data {
		round1Messages = append(round1Messages, data)
	}

	// Process Round 1 and generate Round 2
	msg, err := k.ProcessDKGRound1Messages(sessionID, round1Messages)
	if err != nil {
		fmt.Printf("FROST DKG Round2: failed to process round 1: %v\n", err)
		return nil
	}

	return msg
}

// GenerateSigningCommitmentReal creates real FROST signing Round 1 commitment
func (k Keeper) GenerateSigningCommitmentReal(ctx context.Context, requestID, validatorAddr string) []byte {
	// Get the signing request
	request, err := k.GetSigningRequest(ctx, requestID)
	if err != nil {
		fmt.Printf("FROST Sign Round1: failed to get request: %v\n", err)
		return nil
	}

	// Get the session
	session, err := k.SigningSessionStore.Get(ctx, requestID)
	if err != nil {
		fmt.Printf("FROST Sign Round1: failed to get session: %v\n", err)
		return nil
	}

	// Find our participant index
	participantIndex := -1
	signerIndices := []int{}
	for i, addr := range session.Participants {
		signerIndices = append(signerIndices, i)
		if addr == validatorAddr {
			participantIndex = i
		}
	}
	if participantIndex < 0 {
		fmt.Printf("FROST Sign Round1: validator %s not in participants\n", validatorAddr)
		return nil
	}

	// Initialize FROST sign state if not already done
	// This will load and decrypt key shares from chain on-demand
	if err := k.InitSignState(ctx, requestID, request.KeySetId, participantIndex, signerIndices, request.MessageHash); err != nil {
		fmt.Printf("FROST Sign Round1: failed to init state: %v\n", err)
		return nil
	}

	// Generate Round 1 message (commitment)
	msg, err := k.GenerateSigningRound1Message(requestID, validatorAddr)
	if err != nil {
		fmt.Printf("FROST Sign Round1: failed to generate message: %v\n", err)
		return nil
	}

	return msg
}

// GenerateSignatureShareReal creates real FROST signing Round 2 signature share
func (k Keeper) GenerateSignatureShareReal(ctx context.Context, requestID, validatorAddr string) []byte {
	// Get all Round 1 commitments for this request
	commitments, err := k.AggregateSigningCommitments(ctx, requestID)
	if err != nil {
		fmt.Printf("FROST Sign Round2: failed to get commitments: %v\n", err)
		return nil
	}

	// Collect round 1 messages
	var round1Messages [][]byte
	for _, data := range commitments {
		round1Messages = append(round1Messages, data)
	}

	// Process Round 1 and generate Round 2 (signature share)
	msg, err := k.ProcessSigningRound1Messages(requestID, round1Messages)
	if err != nil {
		fmt.Printf("FROST Sign Round2: failed to process round 1: %v\n", err)
		return nil
	}

	return msg
}
