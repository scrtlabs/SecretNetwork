package mantlemint

import (
	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/state"
	tmtypes "github.com/tendermint/tendermint/types"
	db "github.com/tendermint/tm-db"
	"github.com/enigmampc/SecretNetwork/rumor-go/types"
)

type Mantlemint interface {
	Inject(*types.Block) (*types.BlockState, error)
	Init(*tmtypes.GenesisDoc) error
	GetCurrentHeight() int64
	GetCurrentBlock() *types.Block
	GetCurrentState() state.State
	SetBlockExecutor(executorCreator MantlemintExecutorCreator)
}

type MantlemintExecutor interface {
	ApplyBlock(state.State, tmtypes.BlockID, *types.Block) (state.State, int64, error)
	SetEventBus(publisher tmtypes.BlockEventPublisher)
}

type MantlemintExecutorCreator func(db db.DB, app abcicli.Client) MantlemintExecutor

type Middleware func(conn abcicli.Client) abcicli.Client
