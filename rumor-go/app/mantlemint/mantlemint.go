package mantlemint

import (
	"fmt"
	"log"

	abcicli "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/state"
	tmdb "github.com/tendermint/tm-db"

	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/enigmampc/SecretNetwork/rumor-go/types"
)

var _ Mantlemint = (*MantlemintInstance)(nil)

var (
	errNoBlock = "block is never injected"
)

type MantlemintInstance struct {
	// mem-cached LastState for faster retrieval
	lastState  state.State
	lastHeight int64
	lastBlock  *types.Block
	executor   MantlemintExecutor
	conn       abcicli.Client
	db         tmdb.DB
}

func NewMantlemint(
	db tmdb.DB,
	app abci.Application,
	middlewares ...Middleware,
) Mantlemint {
	// create proxyapp out of terra app,
	// and decorate with middlewares
	conn := NewMantleAppConn(app)
	for _, middleware := range middlewares {
		conn = middleware(conn)
	}

	// here we go!
	return &MantlemintInstance{
		// subsystem
		executor: NewMantlemintExecutor(db, conn),
		db:       db,
		conn:     conn,

		// state related
		lastBlock:  nil,
		lastState:  state.LoadState(db),
		lastHeight: 0,
	}
}

// Init is port of ReplayBlocks() from tendermint,
// where it only handles initializing the chain.
func (mm *MantlemintInstance) Init(genesis *tmtypes.GenesisDoc) error {
	// TODO: move this bit to mantle-compatibility
	// some config
	//config := sdk.GetConfig()
	//config.SetCoinType(core.CoinType)
	//config.SetFullFundraiserPath(core.FullFundraiserPath)
	//config.SetBech32PrefixForAccount(core.Bech32PrefixAccAddr, core.Bech32PrefixAccPub)
	//config.SetBech32PrefixForValidator(core.Bech32PrefixValAddr, core.Bech32PrefixValPub)
	//config.SetBech32PrefixForConsensusNode(core.Bech32PrefixConsAddr, core.Bech32PrefixConsPub)
	//config.Seal()

	// loaded state has LastBlockHeight 0,
	// meaning chain was never initialized
	// run genesis
	log.Printf("genesisTime=%v, chainId=%v", genesis.GenesisTime, genesis.ChainID)

	if mm.lastState.IsEmpty() {
		log.Print("initializing mantle from genesis")

		// create default state from genesis
		var genesisState, err = state.MakeGenesisState(genesis)
		if err != nil {
			return err
		}

		log.Printf("\tgenesisTime=%v, chainId=%v", genesis.GenesisTime, genesis.ChainID)
		log.Printf("\tappHash=%v", genesis.AppHash)

		validators := make([]*tmtypes.Validator, len(genesis.Validators))
		for i, val := range genesis.Validators {
			validators[i] = tmtypes.NewValidator(val.PubKey, val.Power)
		}
		validatorSet := tmtypes.NewValidatorSet(validators)
		nextVals := tmtypes.TM2PB.ValidatorUpdates(validatorSet)

		csParams := tmtypes.TM2PB.ConsensusParams(genesis.ConsensusParams)
		req := abci.RequestInitChain{
			Time:            genesis.GenesisTime,
			ChainId:         genesis.ChainID,
			AppStateBytes:   genesis.AppState,
			ConsensusParams: csParams,
			Validators:      nextVals,
		}

		res, err := mm.conn.InitChainSync(req)

		log.Printf("initChain finished")
		log.Printf("\tvalidators: %v", len(res.Validators))
		log.Printf("\tconsensusParams: %v", res.ConsensusParams)

		if err != nil {
			return err
		}

		// If the app returned validators or consensus params, update the state.
		if len(res.Validators) > 0 {
			vals, err := tmtypes.PB2TM.ValidatorUpdates(res.Validators)
			if err != nil {
				panic(err)
			}
			genesisState.Validators = tmtypes.NewValidatorSet(vals)
			genesisState.NextValidators = tmtypes.NewValidatorSet(vals)
		} else if len(genesis.Validators) == 0 {
			// If validator set is not set in genesis and still empty after InitChain, exit.
			panic(fmt.Errorf("validator set is nil in genesis and still empty after InitChain"))
		}

		if res.ConsensusParams != nil {
			genesisState.ConsensusParams = genesisState.ConsensusParams.Update(res.ConsensusParams)
		}

		// state needs to be saved
		state.SaveState(mm.db, genesisState)

		log.Print("genesis saved to db")

		mm.lastState = genesisState
		mm.lastHeight = 0
	}

	return nil
}

func (mm *MantlemintInstance) Inject(block *types.Block) (*types.BlockState, error) {
	var currentState = mm.lastState
	var blockID = tmtypes.BlockID{
		Hash:        block.Hash(),
		PartsHeader: block.MakePartSet(tmtypes.BlockPartSizeBytes).Header(),
	}

	// apply this block
	var nextState state.State
	var retainHeight int64
	var err error

	// lastBlock must be set before running ApplyBlock
	mm.lastBlock = block

	// patch AppHash of lastState to the current block's last app hash
	// because we still want to use fauxMerkleTree for speed (way faster this way!)
	currentState.AppHash = block.AppHash

	// set new event listener for this round
	// note that we create new event collector for every block,
	// however this operation is quite cheap.
	ev := NewMantlemintEventCollector()
	mm.executor.SetEventBus(ev)

	// process blocks
	if nextState, retainHeight, err = mm.executor.ApplyBlock(currentState, blockID, block); err != nil {
		return nil, err
	}

	// save cache of last state
	mm.lastState = nextState
	mm.lastHeight = retainHeight

	// read events, form blockState and return it
	return ev.GetBlockState(), nil
}

func (mm *MantlemintInstance) GetCurrentHeight() int64 {
	return mm.lastHeight
}

func (mm *MantlemintInstance) GetCurrentBlock() *types.Block {
	if mm.lastBlock == nil {
		panic(errNoBlock)
	}

	return mm.lastBlock
}

func (mm *MantlemintInstance) GetCurrentState() state.State {
	return mm.lastState
}

func (mm *MantlemintInstance) SetBlockExecutor(nextBlockExecutorCreator MantlemintExecutorCreator) {
	mm.executor = nextBlockExecutorCreator(mm.db, mm.conn)
}
