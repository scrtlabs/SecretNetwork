package keeper

import (
	"context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/SecretNetwork/x/registration/internal/types"
	ra "github.com/enigmampc/SecretNetwork/x/registration/remote_attestation"
)

type GrpcQuerier struct {
	keeper Keeper
}

// todo: this needs proper tests and doc
func NewQuerier(keeper Keeper) GrpcQuerier {
	return GrpcQuerier{keeper: keeper}
}

func (q GrpcQuerier) TxKey(c context.Context, req *types.QueryKeyRequest) (*types.Key, error) {
	rsp, err := queryMasterKey(sdk.UnwrapSDKContext(c), q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}

	var keyBytes []byte

	if req.GetKey() {
		keyBytes, err = ra.VerifyRaCert(rsp.IoMasterCertificate.Bytes)
		if err != nil {
			return nil, types.ErrCertificateInvalid
		}
	} else {
		keyBytes = rsp.IoMasterCertificate.Bytes
	}

	return &types.Key{
		Key: keyBytes,
	}, nil
}

func (q GrpcQuerier) RegistrationKey(c context.Context, req *types.QueryKeyRequest) (*types.Key, error) {
	rsp, err := queryMasterKey(sdk.UnwrapSDKContext(c), q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}

	var keyBytes []byte

	if req.GetKey() {
		keyBytes, err = ra.VerifyRaCert(rsp.IoMasterCertificate.Bytes)
		if err != nil {
			return nil, types.ErrCertificateInvalid
		}
	} else {
		keyBytes = rsp.IoMasterCertificate.Bytes
	}

	return &types.Key{
		Key: keyBytes,
	}, nil
}

func (q GrpcQuerier) EncryptedSeed(c context.Context, req *types.QueryEncryptedSeedRequest) (*types.QueryEncryptedSeedResponse, error) {
	if req.PubKey == nil {
		return nil, sdkerrors.Wrap(types.ErrInvalid, "public key")
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
	ioKey := keeper.GetMasterCertificate(ctx, types.MasterIoKeyId)
	nodeKey := keeper.GetMasterCertificate(ctx, types.MasterNodeKeyId)
	if ioKey == nil || nodeKey == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownAddress, "Chain has not been initialized yet")
	}

	resp := &types.GenesisState{
		Registration:              nil,
		NodeExchMasterCertificate: nodeKey,
		IoMasterCertificate:       ioKey,
	}

	//asBytes, err := keeper.cdc.Marshal(ioKey)
	//if err != nil {
	//	return nil, err
	//}

	return resp, nil
}

func queryEncryptedSeed(ctx sdk.Context, pubkeyBytes []byte, keeper Keeper) ([]byte, error) {
	seed := keeper.getRegistrationInfo(ctx, pubkeyBytes)
	if seed == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownAddress, "Node has not been authenticated yet")
	}

	return seed.EncryptedSeed, nil
}
