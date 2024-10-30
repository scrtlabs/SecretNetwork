package keeper

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"

	"cosmossdk.io/core/store"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec"
	types1 "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

// CountTXDecorator ante handler to count the tx position in a block.
type CountTXDecorator struct {
	appcodec     codec.Codec
	govkeeper    govkeeper.Keeper // we need the govkeeper to access stored proposals
	storeService store.KVStoreService
}

const msgSoftwareUpgradeTypeURL = "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade"

// NewCountTXDecorator constructor
func NewCountTXDecorator(appcodec codec.Codec, govkeeper govkeeper.Keeper, storeService store.KVStoreService) *CountTXDecorator {
	return &CountTXDecorator{
		appcodec:     appcodec,
		govkeeper:    govkeeper,
		storeService: storeService,
	}
}

// Function to find and return the MREnclaveHash string from input
func findMREnclaveHash(input string) ([]byte, error) {
	// Define the regular expression pattern with a capture group for the SHA256 hash
	pattern := `^MREnclaveHash:([a-fA-F0-9]{64})$`

	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(input)

	// If no match is found, return an error
	if len(matches) < 2 {
		return nil, errors.New("MREnclaveHash not found or invalid in the input string")
	}

	mrEnclaveHash, err := hex.DecodeString(matches[1])
	if err != nil {
		return nil, err
	}

	// The SHA256 hash is captured in the first capturing group, which is matches[1]
	return mrEnclaveHash, nil
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
			err = a.verifyUpgradeProposal(ctx, msgUpgrade)
			if err != nil {
				fmt.Println("*** upgrade proposal pass rejected: ", err)
				return ctx, err
			}
		}
	}

	return next(types.WithTXCounter(ctx, txCounter), tx, simulate)
}

// extractInfoFromProposalMessages extracts the "info" field from the proposal message.
// This "info" contains the MREnclaveHash.
func extractInfoFromProposalMessages(message *types1.Any, cdc codec.Codec) (string, error) {
	var softwareUpgradeMsg *upgradetypes.MsgSoftwareUpgrade
	err := cdc.UnpackAny(message, &softwareUpgradeMsg)
	if err != nil {
		return "", fmt.Errorf("failed to unpack message: %w", err)
	}

	return softwareUpgradeMsg.Plan.Info, nil
}

// verifyUpgradeProposal verifies the latest passed upgrade proposal to ensure the MREnclave hash matches.
func (a *CountTXDecorator) verifyUpgradeProposal(ctx sdk.Context, msgUpgrade *types.MsgUpgradeProposalPassed) error {
	var proposals govtypes.Proposals
	err := a.govkeeper.Proposals.Walk(ctx, nil, func(_ uint64, value govtypes.Proposal) (stop bool, err error) {
		proposals = append(proposals, &value)
		return false, nil
	})
	if err != nil {
		ctx.Logger().Error("gov keeper", "proposal", err.Error())
		return err
	}

	var latestProposal *v1.Proposal = nil
	var latestMREnclaveHash []byte

	// Iterate through the proposals
	for _, proposal := range proposals {
		// Check if the proposal has passed and is of type MsgSoftwareUpgrade
		if proposal.Status == v1.ProposalStatus_PROPOSAL_STATUS_PASSED {
			if len(proposal.GetMessages()) > 0 && proposal.Messages[0].GetTypeUrl() == msgSoftwareUpgradeTypeURL {
				// Update latestProposal if this proposal is newer (has a higher ID)
				if latestProposal == nil || proposal.Id > latestProposal.Id {
					latestProposal = proposal
				}
			}
		}
	}

	if latestProposal == nil {
		return fmt.Errorf("no latest upgrade proposal")
	}

	// If we found the MsgSoftwareUpgrade latest passed proposal, extract the MREnclaveHash from it
	info, err := extractInfoFromProposalMessages(latestProposal.Messages[0], a.appcodec)
	if err != nil {
		return fmt.Errorf("Failed to extract info with MREnclave hash from Proposal, error: %w", err)
	}
	latestMREnclaveHash, _ = findMREnclaveHash(info)
	if latestMREnclaveHash == nil {
		return fmt.Errorf("no mrenclave in the latest upgrade proposal")
	}

	// Check if the MREnclave hash matches the one in the MsgUpgradeProposalPassed message
	if !bytes.Equal(latestMREnclaveHash, msgUpgrade.MrEnclaveHash) {
		return sdkerrors.ErrUnauthorized.Wrap("software upgrade proposal: mrenclave hash mismatch")
	}
	return nil
}

func encodeHeightCounter(height int64, counter uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, counter)
	return append(sdk.Uint64ToBigEndian(uint64(height)), b...)
}

func decodeHeightCounter(bz []byte) (int64, uint32) {
	return int64(sdk.BigEndianToUint64(bz[0:8])), binary.BigEndian.Uint32(bz[8:])
}
