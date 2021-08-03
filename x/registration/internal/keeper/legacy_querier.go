package keeper

import (
	"encoding/hex"
	"encoding/json"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	abci "github.com/tendermint/tendermint/abci/types"
	"reflect"
)

const (
	QueryEncryptedSeed     = "seed"
	QueryMasterCertificate = "master-cert"
)

// controls error output on querier - set true when testing/debugging
const debug = false

// NewQuerier creates a new querier
func NewLegacyQuerier(keeper Keeper) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) ([]byte, error) {
		var (
			rsp interface{}
			err error
			bz  []byte
		)
		switch path[0] {
		case QueryEncryptedSeed:
			pubKey, err := hex.DecodeString(path[1])
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
			}
			bz, err = queryEncryptedSeed(ctx, pubKey, keeper)
			if err != nil {
				return nil, err
			}
			return bz, nil
		case QueryMasterCertificate:
			rsp, err = queryMasterKey(ctx, keeper)

			if err != nil {
				return nil, err
			}

			if rsp == nil || reflect.ValueOf(rsp).IsNil() {
				return nil, nil
			}
			// why indent?
			bz, err = json.Marshal(rsp)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
			}
			return bz, nil
		default:
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "unknown data query endpoint")
		}
	}
}
