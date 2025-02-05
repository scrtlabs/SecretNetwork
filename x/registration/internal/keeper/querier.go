package keeper

import (
	"context"
	"encoding/json"

	"github.com/golang/protobuf/ptypes/empty"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
)

const (
	QueryEncryptedSeed = "seed"
	QueryMasterKey     = "master-key"
)

type GrpcQuerier struct {
	keeper Keeper
}

// todo: this needs proper tests and doc
func NewQuerier(keeper Keeper) GrpcQuerier {
	return GrpcQuerier{keeper: keeper}
}

func (q GrpcQuerier) TxKey(c context.Context, _ *empty.Empty) (*types.Key, error) {
	rsp, err := queryMasterKey(sdk.UnwrapSDKContext(c), q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}

	return &types.Key{
		Key: rsp.IoMasterKey.Bytes,
	}, nil
}

func (q GrpcQuerier) RegistrationKey(c context.Context, _ *empty.Empty) (*types.Key, error) {
	rsp, err := queryMasterKey(sdk.UnwrapSDKContext(c), q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	keys, err := json.Marshal(rsp)
	if err != nil {
		return nil, err
	}
	return &types.Key{
		Key: keys,
	}, nil
}

func (q GrpcQuerier) EncryptedSeed(c context.Context, req *types.QueryEncryptedSeedRequest) (*types.QueryEncryptedSeedResponse, error) {
	if req.PubKey == nil {
		return nil, errorsmod.Wrap(types.ErrInvalid, "public key")
	}
	rsp, err := queryEncryptedSeed(sdk.UnwrapSDKContext(c), req.PubKey, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QueryEncryptedSeedResponse{EncryptedSeed: rsp}, nil
}

func queryMasterKey(ctx sdk.Context, keeper Keeper) (*types.GenesisState, error) {
	ioKey := keeper.GetMasterKey(ctx, types.MasterIoKeyId)
	nodeKey := keeper.GetMasterKey(ctx, types.MasterNodeKeyId)
	if ioKey == nil || nodeKey == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrUnknownAddress, "Chain has not been initialized yet")
	}

	resp := &types.GenesisState{
		Registration:      nil,
		NodeExchMasterKey: nodeKey,
		IoMasterKey:       ioKey,
	}

	return resp, nil
}

func queryEncryptedSeed(ctx sdk.Context, pubkeyBytes []byte, keeper Keeper) ([]byte, error) {
	seed := keeper.getRegistrationInfo(ctx, pubkeyBytes)
	if seed == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrUnknownAddress, "Node has not been authenticated yet")
	}

	return seed.EncryptedSeed, nil
}
