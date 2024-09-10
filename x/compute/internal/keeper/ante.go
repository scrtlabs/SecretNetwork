package keeper

import (
	"encoding/binary"
	"errors"
	"regexp"

	"cosmossdk.io/core/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"

	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
)

// CountTXDecorator ante handler to count the tx position in a block.
type CountTXDecorator struct {
	storeService store.KVStoreService
	govkeeper    govkeeper.Keeper // we need the govkeeper to access stored proposals
}

// NewCountTXDecorator constructor
func NewCountTXDecorator(storeService store.KVStoreService, govkeeper govkeeper.Keeper) *CountTXDecorator {
	return &CountTXDecorator{
		storeService: storeService,
		govkeeper:    govkeeper,
	}
}

// Function to find and return the MREnclaveHash string from input
func findMREnclaveHash(input string) (string, error) {
	// Define the regular expression pattern with a capture group for the SHA256 hash
	pattern := `^MREnclaveHash:([a-fA-F0-9]{64})$`

	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(input)

	// If no match is found, return an error
	if len(matches) < 2 {
		return "", errors.New("MREnclaveHash not found or invalid in the input string")
	}

	// The SHA256 hash is captured in the first capturing group, which is matches[1]
	return matches[1], nil
}

// AnteHandle handler stores a tx counter with current height encoded in the store to let the app handle
// global rollback behavior instead of keeping state in the handler itself.
// The ante handler passes the counter value via sdk.Context upstream. See `types.TXCounter(ctx)` to read the value.
// Simulations don't get a tx counter value assigned.
func (a CountTXDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if simulate {
		return next(ctx, tx, simulate)
	}
	store := a.storeService.OpenKVStore(ctx)
	currentHeight := ctx.BlockHeight()

	var txCounter uint32 // start with 0
	// load counter when exists
	if bz, _ := store.Get(types.TXCounterPrefix); bz != nil {
		lastHeight, val := decodeHeightCounter(bz)
		if currentHeight == lastHeight {
			// then use stored counter
			txCounter = val
		} // else use `0` from above to start with
	}
	// store next counter value for current height
	err := store.Set(types.TXCounterPrefix, encodeHeightCounter(currentHeight, txCounter+1))
	if err != nil {
		ctx.Logger().Error("compute ante store set", "store", err.Error())
	}

	for _, msg := range tx.GetMsgs() {
		// Check if this is a MsgUpgradeProposalPassed
		msgUpgrade, ok := msg.(*types.MsgUpgradeProposalPassed)
		if ok {
			iterator, err := a.govkeeper.Proposals.Iterate(ctx, nil)
			if err != nil {
				ctx.Logger().Error("Failed to get the iterator of proposals!", err.Error())
			}

			defer iterator.Close() // Ensure the iterator is closed after use

			var latestProposal *v1.Proposal
			var latestMREnclaveHash string

			// Iterate through the proposals
			for ; iterator.Valid(); iterator.Next() {
				// Get the proposal value
				proposal, err := iterator.Value()
				if err != nil {
					ctx.Logger().Error("Failed to get the proposal from iterator!", err.Error())
				}

				mrenclaveHash, err := findMREnclaveHash(proposal.Metadata)
				// Apply filter: Check if the proposal has "passed"
				if err == nil && proposal.Status == v1.ProposalStatus_PROPOSAL_STATUS_PASSED {
					// Check if this is the latest passed proposal by id
					if latestProposal == nil || proposal.Id > latestProposal.Id {
						latestProposal = &proposal
						latestMREnclaveHash = mrenclaveHash
					}
				}
			}

			// Retrieve the stored mrenclave hash from the keeper
			if latestMREnclaveHash != string(msgUpgrade.MrEnclaveHash) {
				return ctx, sdkerrors.ErrUnauthorized.Wrap("mrenclave hash mismatch")
			}
		}
	}

	return next(types.WithTXCounter(ctx, txCounter), tx, simulate)
}

func encodeHeightCounter(height int64, counter uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, counter)
	return append(sdk.Uint64ToBigEndian(uint64(height)), b...)
}

func decodeHeightCounter(bz []byte) (int64, uint32) {
	return int64(sdk.BigEndianToUint64(bz[0:8])), binary.BigEndian.Uint32(bz[8:])
}
