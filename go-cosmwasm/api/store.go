package api

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/cache"
	"github.com/cosmos/cosmos-sdk/store/iavl"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

type storeWithParent interface{ GetParent() sdk.KVStore }

func getInnerIavl(store sdk.KVStore, key []byte) (iavlStore *iavl.Store, fullKey []byte, err error) {
	switch st := store.(type) {
	case prefix.Store: // Special case for a prefix store to get the prefixed key
		return getInnerIavl(st.GetParent(), st.Key(key))
	case storeWithParent:
		return getInnerIavl(st.GetParent(), key)
	case *cache.CommitKVStoreCache:
		return getInnerIavl(st.CommitKVStore, key)
	case *iavl.Store:
		return st, key, nil
	default:
		return nil, nil, fmt.Errorf("store type not supported: %+v", store)
	}
}

func getWithProof(store sdk.KVStore, key []byte, blockHeight int64) (value []byte, proof []byte, fullKey []byte, err error) {
	iavlStore, fullKey, err := getInnerIavl(store, key)
	if err != nil {
		return nil, nil, nil, err
	}

	// Query height is (current - 1) because we will probably not have a proof in
	// the current height (assuming we're mid execution)
	result := iavlStore.Query(abci.RequestQuery{Data: fullKey, Path: "/key", Prove: true, Height: blockHeight - 1})

	// result.ProofOps.Ops should always contain only one proof
	if result.ProofOps == nil {
		return nil, nil, nil, fmt.Errorf("error in retrieving key: %+v for height: %d, got: %s", key, blockHeight-1, result.Log)
	}
	if len(result.ProofOps.Ops) != 1 {
		return nil, nil, nil, fmt.Errorf("error in retrieving proof for key: %+v, got: %+v", key, result.ProofOps.Ops)
	}
	if result.ProofOps.Ops[0].Data == nil {
		return nil, nil, nil, fmt.Errorf("`iavlStore.Query()` returned an empty value for key: %+v", key)
	}

	return result.Value, result.ProofOps.Ops[0].Data, fullKey, nil
}
