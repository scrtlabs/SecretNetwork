package api

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/cache"
	"github.com/cosmos/cosmos-sdk/store/iavl"
	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

type storeWithParent interface{ GetParent() sdk.KVStore }

func getIavl(store sdk.KVStore) (*iavl.Store, error) {
	switch st := store.(type) {
	case storeWithParent:
		return getIavl(st.GetParent())
	case *cache.CommitKVStoreCache:
		return getIavl(st.CommitKVStore)
	case *iavl.Store:
		return st, nil
	default:
		return nil, fmt.Errorf("store type not supported: %+v", store)
	}
}

func getWithProof(store sdk.KVStore, key []byte, blockHeight int64) (value []byte, proof []byte, err error) {
	iavlStore, err := getIavl(store)
	if err != nil {
		return nil, nil, err
	}

	// Query height is (current - 1) because we will probably not have a proof in
	// the current height (assuming we're mid execution)
	result := iavlStore.Query(abci.RequestQuery{Data: key, Path: "/key", Prove: true, Height: blockHeight - 1 /* NOTE!! this depends on what blockHeight we get here. if it's the verified one it's probably for the past block anyway, so no need to subtract 1 for the height*/})

	// result.ProofOps.Ops should always contain only one proof
	if len(result.ProofOps.Ops) != 1 {
		return nil, nil, fmt.Errorf("error in retrieving proof for key: %+v, got: %+v", key, result.ProofOps.Ops)
	}

	return result.Value, result.ProofOps.Ops[0].Data, nil
}
