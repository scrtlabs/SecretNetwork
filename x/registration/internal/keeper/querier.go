package keeper

import (
	"encoding/hex"
	"encoding/json"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/EnigmaBlockchain/x/registration/internal/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

const (
	QueryEncryptedSeed     = "seed"
	QueryMasterCertificate = "master-cert"
)

// controls error output on querier - set true when testing/debugging
const debug = false

// NewQuerier creates a new querier
func NewQuerier(keeper Keeper) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) ([]byte, error) {
		switch path[0] {
		case QueryEncryptedSeed:
			return queryEncryptedSeed(ctx, path[1], req, keeper)
		case QueryMasterCertificate:
			return queryMasterKey(ctx, req, keeper)
		default:
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "unknown data query endpoint")
		}
	}
}

func queryMasterKey(ctx sdk.Context, req abci.RequestQuery, keeper Keeper) ([]byte, error) {
	ioKey := keeper.GetMasterCertificate(ctx, types.MasterIoKeyId)
	nodeKey := keeper.GetMasterCertificate(ctx, types.MasterNodeKeyId)
	if ioKey == nil || nodeKey == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownAddress, "Chain has not been initialized yet")
	}

	resp := types.GenesisState{
		Registration:              nil,
		NodeExchMasterCertificate: *nodeKey,
		IoMasterCertificate:       *ioKey,
	}

	asBytes, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	return asBytes, nil
}

func queryEncryptedSeed(ctx sdk.Context, pubkey string, req abci.RequestQuery, keeper Keeper) ([]byte, error) {
	pubkeyBytes, err := hex.DecodeString(pubkey)
	if err != nil {
		return nil, sdkerrors.Wrap(types.ErrInvalidType, err.Error())
	}

	seed := keeper.getRegistrationInfo(ctx, pubkeyBytes)
	if seed == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownAddress, "Node has not been authenticated yet")
	}

	return seed.EncryptedSeed, nil
}
