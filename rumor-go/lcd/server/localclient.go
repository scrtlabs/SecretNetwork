package server

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/bytes"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// MUST implement rpcclient.Client
type LocalClient struct {
	App abci.Application
	mtx *sync.Mutex
	rpcclient.Client
}

func NewLocalClient(app abci.Application, m *sync.Mutex) LocalClient {
	return LocalClient{
		App: app,
		mtx: m,
	}
}

func (c LocalClient) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) ABCIQuery(path string, data bytes.HexBytes) (*ctypes.ResultABCIQuery, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return &ctypes.ResultABCIQuery{
		Response: c.App.Query(abci.RequestQuery{
			Data: data,
			Path: path,
		}),
	}, nil
}

func (c LocalClient) ABCIQueryWithOptions(path string, data bytes.HexBytes, opts rpcclient.ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return &ctypes.ResultABCIQuery{
		Response: c.App.Query(abci.RequestQuery{
			Data:   data,
			Path:   path,
			Height: opts.Height,
			Prove:  opts.Prove,
		}),
	}, nil
}

func (c LocalClient) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) Block(height *int64) (*ctypes.ResultBlock, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) BlockResults(height *int64) (*ctypes.ResultBlockResults, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) Commit(height *int64) (*ctypes.ResultCommit, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) Validators(height *int64, papge, perPage int) (*ctypes.ResultValidators, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) TxSearch(query string, prove bool, page, perPage int, orderBy string) (*ctypes.ResultTxSearch, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) Genesis() (*ctypes.ResultGenesis, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) BlockchainInfo(minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) Status() (*ctypes.ResultStatus, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) NetInfo() (*ctypes.ResultNetInfo, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) ConsensusState() (*ctypes.ResultConsensusState, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) Health() (*ctypes.ResultHealth, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) Subscribe(ctx context.Context, subscriber, query string, outCapacity ...int) (out <-chan ctypes.ResultEvent, err error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) Unsubscribe(ctx context.Context, subscriber, query string) error {
	invariant()
	return fmt.Errorf("Not implemented")
}

func (c LocalClient) UnsubscribeAll(ctx context.Context, subscriber string) error {
	invariant()
	return fmt.Errorf("Not implemented")
}

func (c LocalClient) UnconfirmedTxs(limit int) (*ctypes.ResultUnconfirmedTxs, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) NumUnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func (c LocalClient) BroadcastEvidence(ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	invariant()
	return nil, fmt.Errorf("Not implemented")
}

func invariant() {
	pc, _, _, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)
	fmt.Println(fmt.Sprintf("%s is called, however it is not supported by mantle", fn.Name()))
}
