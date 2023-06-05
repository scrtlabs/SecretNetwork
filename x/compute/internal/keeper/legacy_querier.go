package keeper

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

const (
	QueryListContractByCode   = "list-contracts-by-code"
	QueryGetContract          = "contract-info"
	QueryGetContractState     = "contract-state"
	QueryGetCode              = "code"
	QueryListCode             = "list-code"
	QueryContractAddress      = "label"
	QueryContractKey          = "contract-key"
	QueryContractHash         = "contract-hash"
	QueryContractHashByCodeID = "contract-hash-by-id"
)

const QueryMethodContractStateSmart = "smart"

// NewLegacyQuerier creates a new querier
func NewLegacyQuerier(keeper Keeper) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) ([]byte, error) {
		var (
			rsp interface{}
			err error
			bz  []byte
		)
		switch path[0] {
		case QueryGetContract:
			addr, err := sdk.AccAddressFromBech32(path[1])
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
			}
			rsp, err = queryContractInfo(ctx, addr, keeper)
			if err != nil {
				return nil, sdkerrors.Wrapf(types.ErrInvalid, "contract id: %s", err.Error())
			}
		case QueryListContractByCode:
			codeID, err := strconv.ParseUint(path[1], 10, 64)
			if err != nil {
				return nil, sdkerrors.Wrapf(types.ErrInvalid, "code id: %s", err.Error())
			}
			rsp, err = queryContractListByCode(ctx, codeID, keeper)
			if err != nil {
				return nil, sdkerrors.Wrapf(types.ErrInvalid, "code id: %s", err.Error())
			}
		case QueryGetContractState:
			if len(path) < 2 {
				return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, fmt.Sprintf("%s too few arguments (wanted at least 2): %v", QueryGetContractState, path))
			}
			return queryContractState(ctx, path[1], "unused" /* path[2] */, req.Data, keeper)
		case QueryGetCode:
			codeID, err := strconv.ParseUint(path[1], 10, 64)
			if err != nil {
				return nil, sdkerrors.Wrapf(types.ErrInvalid, "code id: %s", err.Error())
			}
			rsp, err = queryCode(ctx, codeID, keeper)
			if err != nil {
				return nil, sdkerrors.Wrapf(types.ErrInvalid, "code id: %s", err.Error())
			}
		case QueryListCode:
			rsp, err = queryCodeList(ctx, keeper)
		case QueryContractAddress:
			bz, err = queryContractAddress(ctx, path[1], keeper)
			// return rsp, nil
		case QueryContractKey:
			addr, err := sdk.AccAddressFromBech32(path[1])
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
			}
			bz, err = queryContractKey(ctx, addr, keeper)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
			}
		case QueryContractHash:
			addr, err := sdk.AccAddressFromBech32(path[1])
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
			}
			bz, err = queryCodeHashByAddress(ctx, addr, keeper)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
			}
		case QueryContractHashByCodeID:
			codeID, err := strconv.ParseUint(path[1], 10, 64)
			if err != nil {
				return nil, sdkerrors.Wrapf(types.ErrInvalid, "code id: %s", err.Error())
			}
			bz, err = queryCodeHashByCodeID(ctx, codeID, keeper)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
			}
		default:
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, fmt.Sprintf("unknown data query endpoint %s", path[0]))
		}
		if err != nil {
			return nil, err
		}

		if bz != nil {
			return bz, nil
		}

		if rsp == nil || reflect.ValueOf(rsp).IsNil() {
			return nil, nil
		}

		bz, err = json.MarshalIndent(rsp, "", "    ")
		if err != nil {
			return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
		}
		return bz, nil
	}
}

func queryContractState(ctx sdk.Context, bech, queryMethod string, data []byte, keeper Keeper) (json.RawMessage, error) { //nolint:all
	contractAddr, err := sdk.AccAddressFromBech32(bech)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, bech)
	}

	// we enforce a subjective gas limit on all queries to avoid infinite loops
	ctx = ctx.WithGasMeter(sdk.NewGasMeter(keeper.queryGasLimit))
	// this returns raw bytes (must be base64-encoded)
	return keeper.QuerySmart(ctx, contractAddr, data, false)
}

func queryContractKey(ctx sdk.Context, address sdk.AccAddress, keeper Keeper) ([]byte, error) {
	res, err := keeper.GetContractKey(ctx, address)
	if err != nil {
		return nil, nil
	}

	return res.Key, nil
}
