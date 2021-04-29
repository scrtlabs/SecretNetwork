package mantlemint

import (
	"fmt"
	abcicli "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	tmstate "github.com/tendermint/tendermint/state"
	tm "github.com/tendermint/tendermint/types"
	tmdb "github.com/tendermint/tm-db"
	"github.com/enigmampc/SecretNetwork/rumor-go/types"
)

// type check
var _ MantlemintExecutor = (*SimBlockExecutor)(nil)
var _ MantlemintExecutorCreator = NewSimBlockExecutor

type SimBlockExecutor struct {
	db       tmdb.DB
	conn     abcicli.Client
	eventBus tm.BlockEventPublisher
}

func NewSimBlockExecutor(
	db tmdb.DB,
	conn abcicli.Client,
) MantlemintExecutor {
	return &SimBlockExecutor{
		db:   db,
		conn: conn,
	}
}

func (sbe *SimBlockExecutor) ApplyBlock(
	state tmstate.State,
	blockID tm.BlockID,
	block *types.Block,
) (tmstate.State, int64, error) {
	// closely mock BlockExecutor.ApplyBlock

	// exec block
	abciResponses, err := execBlockOnProxyApp(sbe.conn, block, sbe.db)
	if err != nil {
		return state, 0, err
	}

	// save ABCI
	tmstate.SaveABCIResponses(sbe.db, block.Height, abciResponses)

	// TODO: update validator updates
	abciValUpdates := abciResponses.EndBlock.ValidatorUpdates
	validatorUpdates, err := tm.PB2TM.ValidatorUpdates(abciValUpdates)
	if err != nil {
		return state, 0, err
	}

	// Update the state with the block and responses.
	state, err = updateState(state, blockID, &block.Header, abciResponses, validatorUpdates)
	if err != nil {
		return state, 0, fmt.Errorf("commit failed for application: %v", err)
	}

	// commit
	responseCommit, commitErr := sbe.conn.CommitSync()
	if commitErr != nil {
		return state, 0, commitErr
	}

	// update app hash (not needed as mantle doesn't care, but just in case)
	// save state
	state.AppHash = responseCommit.Data
	tmstate.SaveState(sbe.db, state)

	// fire events
	sbe.eventBus.PublishEventNewBlock(tm.EventDataNewBlock{
		Block:            block,
		ResultBeginBlock: *abciResponses.BeginBlock,
		ResultEndBlock:   *abciResponses.EndBlock,
	})

	sbe.eventBus.PublishEventNewBlockHeader(tm.EventDataNewBlockHeader{
		Header:           block.Header,
		NumTxs:           int64(len(block.Txs)),
		ResultBeginBlock: *abciResponses.BeginBlock,
		ResultEndBlock:   *abciResponses.EndBlock,
	})

	for i, tx := range block.Data.Txs {
		sbe.eventBus.PublishEventTx(tm.EventDataTx{
			TxResult: tm.TxResult{
				Height: block.Height,
				Index:  uint32(i),
				Tx:     tx,
				Result: *(abciResponses.DeliverTxs[i]),
			},
		})
	}

	if len(validatorUpdates) > 0 {
		sbe.eventBus.PublishEventValidatorSetUpdates(
			tm.EventDataValidatorSetUpdates{ValidatorUpdates: validatorUpdates})
	}

	return state, responseCommit.RetainHeight, nil
}

func (sbe *SimBlockExecutor) SetEventBus(publisher tm.BlockEventPublisher) {
	sbe.eventBus = publisher
}

func execBlockOnProxyApp(
	app abcicli.Client,
	block *types.Block,
	db tmdb.DB,
) (*tmstate.ABCIResponses, error) {
	abciResponses := tmstate.NewABCIResponses(block)

	// begin block validator info
	commitInfo, byzVals := getBeginBlockValidatorInfoSim(block, db)

	// begin block
	beginBlockerResult, err := app.BeginBlockSync(abci.RequestBeginBlock{
		Hash:                block.Hash(),
		Header:              tm.TM2PB.Header(&block.Header),
		LastCommitInfo:      commitInfo,
		ByzantineValidators: byzVals,
	})
	if err != nil {
		return nil, err
	}
	abciResponses.BeginBlock = beginBlockerResult

	// deliver txs
	for txi, tx := range block.Data.Txs {
		responseDeliverTx, err := app.DeliverTxSync(abci.RequestDeliverTx{
			Tx: tx,
		})
		if err != nil {
			return nil, err
		}
		abciResponses.DeliverTxs[txi] = responseDeliverTx
	}

	// endblock
	responseEndblock, err := app.EndBlockSync(abci.RequestEndBlock{
		Height: block.Height,
	})
	if err != nil {
		return nil, err
	}

	abciResponses.EndBlock = responseEndblock

	return abciResponses, nil
}

func getBeginBlockValidatorInfoSim(block *types.Block, stateDB tmdb.DB) (abci.LastCommitInfo, []abci.Evidence) {
	voteInfos := make([]abci.VoteInfo, block.LastCommit.Size())

	if block.Height > 1 {
		lastValSet, err := tmstate.LoadValidators(stateDB, block.Height-1)
		if err != nil {
			panic(err)
		}

		// Sanity check that commit size matches validator set size - only applies
		// after first block.
		var (
			commitSize = block.LastCommit.Size()
			valSetLen  = len(lastValSet.Validators)
		)
		if commitSize != valSetLen {
			panic(fmt.Sprintf("commit size (%d) doesn't match valset length (%d) at height %d\n\n%v\n\n%v",
				commitSize, valSetLen, block.Height, block.LastCommit.Signatures, lastValSet.Validators))
		}

		for i, val := range lastValSet.Validators {
			commitSig := block.LastCommit.Signatures[i]
			voteInfos[i] = abci.VoteInfo{
				Validator:       tm.TM2PB.Validator(val),
				SignedLastBlock: !commitSig.Absent(),
			}
		}
	}

	byzVals := make([]abci.Evidence, len(block.Evidence.Evidence))
	for i, ev := range block.Evidence.Evidence {
		// We need the validator set. We already did this in validateBlock.
		// TODO: Should we instead cache the valset in the evidence itself and add
		// `SetValidatorSet()` and `ToABCI` methods ?
		valset, err := tmstate.LoadValidators(stateDB, ev.Height())
		if err != nil {
			panic(err)
		}
		byzVals[i] = tm.TM2PB.Evidence(ev, valset, block.Time)
	}

	return abci.LastCommitInfo{
		Round: int32(block.LastCommit.Round),
		Votes: voteInfos,
	}, byzVals
}

// updateState returns a new State updated according to the header and responses.
func updateState(
	state tmstate.State,
	blockID tm.BlockID,
	header *types.Header,
	abciResponses *tmstate.ABCIResponses,
	validatorUpdates []*tm.Validator,
) (tmstate.State, error) {

	// Copy the valset so we can apply changes from EndBlock
	// and update s.LastValidators and s.Validators.
	nValSet := state.NextValidators.Copy()

	// Update the validator set with the latest abciResponses.
	lastHeightValsChanged := state.LastHeightValidatorsChanged
	if len(validatorUpdates) > 0 {
		err := nValSet.UpdateWithChangeSet(validatorUpdates)
		if err != nil {
			return state, fmt.Errorf("error changing validator set: %v", err)
		}
		// Change results from this height but only applies to the next next height.
		lastHeightValsChanged = header.Height + 1 + 1
	}

	// Update validator proposer priority and set state variables.
	nValSet.IncrementProposerPriority(1)

	// Update the params with the latest abciResponses.
	nextParams := state.ConsensusParams
	lastHeightParamsChanged := state.LastHeightConsensusParamsChanged
	if abciResponses.EndBlock.ConsensusParamUpdates != nil {
		// NOTE: must not mutate s.ConsensusParams
		nextParams = state.ConsensusParams.Update(abciResponses.EndBlock.ConsensusParamUpdates)
		err := nextParams.Validate()
		if err != nil {
			return state, fmt.Errorf("error updating consensus params: %v", err)
		}
		// Change results from this height but only applies to the next height.
		lastHeightParamsChanged = header.Height + 1
	}

	// TODO: allow app to upgrade version
	nextVersion := state.Version

	// NOTE: the AppHash has not been populated.
	// It will be filled on state.Save.
	return tmstate.State{
		Version:                          nextVersion,
		ChainID:                          state.ChainID,
		LastBlockHeight:                  header.Height,
		LastBlockID:                      blockID,
		LastBlockTime:                    header.Time,
		NextValidators:                   nValSet,
		Validators:                       state.NextValidators.Copy(),
		LastValidators:                   state.Validators.Copy(),
		LastHeightValidatorsChanged:      lastHeightValsChanged,
		ConsensusParams:                  nextParams,
		LastHeightConsensusParamsChanged: lastHeightParamsChanged,
		LastResultsHash:                  abciResponses.ResultsHash(),
		AppHash:                          nil,
	}, nil
}
