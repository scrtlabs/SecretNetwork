package keeper

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

const (
	QueryListContractByCode = "list-contracts-by-code"
	QueryGetContract        = "contract-info"
	QueryGetContractState   = "contract-state"
	QueryGetCode            = "code"
	QueryListCode           = "list-code"
	QueryContractAddress    = "label"
	QueryContractKey        = "contract-key"
	QueryContractHash       = "contract-hash"
	//QueryContractHistory    = "contract-history"
)

const QueryMethodContractStateSmart = "smart"

/*
const (
	QueryMethodContractStateSmart = "smart"
	QueryMethodContractStateAll   = "all"
	QueryMethodContractStateRaw   = "raw"
)
*/

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
		case QueryListContractByCode:
			codeID, err := strconv.ParseUint(path[1], 10, 64)
			if err != nil {
				return nil, sdkerrors.Wrapf(types.ErrInvalid, "code id: %s", err.Error())
			}
			rsp, err = queryContractListByCode(ctx, codeID, keeper)
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
		case QueryListCode:
			rsp, err = queryCodeList(ctx, keeper)
		/*
			case QueryContractHistory:
				contractAddr, err := sdk.AccAddressFromBech32(path[1])
				if err != nil {
					return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
				}
				rsp, err = queryContractHistory(ctx, contractAddr, keeper)
		*/
		case QueryContractAddress:
			bz, err = queryContractAddress(ctx, path[1], keeper)
			// return rsp, nil
		case QueryContractKey:
			addr, err := sdk.AccAddressFromBech32(path[1])
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
			}
			bz, err = queryContractKey(ctx, addr, keeper)
		case QueryContractHash:
			addr, err := sdk.AccAddressFromBech32(path[1])
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
			}
			bz, err = queryContractHash(ctx, addr, keeper)
		default:
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, fmt.Sprintf("unknown data query endpoint %v", path[0]))
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

		//bz, err = keeper.legacyAmino.MarshalJSON(rsp)
		bz, err = json.MarshalIndent(rsp, "", "  ")
		if err != nil {
			return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
		}
		return bz, nil
	}
}

func queryContractState(ctx sdk.Context, bech, queryMethod string, data []byte, keeper Keeper) (json.RawMessage, error) {
	contractAddr, err := sdk.AccAddressFromBech32(bech)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, bech)
	}

	/*
		var resultData []types.Model
			switch queryMethod {
			case QueryMethodContractStateAll:
				// this returns a serialized json object (which internally encoded binary fields properly)
				for iter := keeper.GetContractState(ctx, contractAddr); iter.Valid(); iter.Next() {
					resultData = append(resultData, types.Model{
						Key:   iter.Key(),
						Value: iter.Value(),
					})
				}
				if resultData == nil {
					resultData = make([]types.Model, 0)
				}
			case QueryMethodContractStateRaw:
				// this returns the raw data from the state, base64-encoded
				return keeper.QueryRaw(ctx, contractAddr, data), nil

			case QueryMethodContractStateSmart:
	*/

	// we enforce a subjective gas limit on all queries to avoid infinite loops
	ctx = ctx.WithGasMeter(sdk.NewGasMeter(keeper.queryGasLimit))
	// this returns raw bytes (must be base64-encoded)
	return keeper.QuerySmart(ctx, contractAddr, data, false)

	/*
			default:
				return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, queryMethod)
			}


		bz, err := json.Marshal(resultData)
		if err != nil {
			return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
		}
		return bz, nil
	*/

}
