package mantlemint

import (
	"io/ioutil"
	"sync"

	abcicli "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/mock"
	"github.com/tendermint/tendermint/state"
	tmdb "github.com/tendermint/tm-db"
)

func NewMantlemintExecutor(
	db tmdb.DB,
	AppConn abcicli.Client,
) *state.BlockExecutor {
	return state.NewBlockExecutor(
		db,
		log.NewTMLogger(ioutil.Discard),
		AppConn,
		mock.Mempool{},           // no mempool
		state.MockEvidencePool{}, // no evidence pool
	)
}

func NewMantlemintSimulationExecutor(
	db tmdb.DB,
	AppConn abcicli.Client,
) *state.BlockExecutor {
	return state.NewBlockExecutor(
		db,
		log.NewTMLogger(ioutil.Discard),
		AppConn,
		mock.Mempool{},           // no mempool
		state.MockEvidencePool{}, // no evidence pool
	)
}

func NewMantleAppConn(
	app abci.Application,
) abcicli.Client {
	mtx := new(sync.Mutex)
	return abcicli.NewLocalClient(mtx, app)
}
