package keeper

import (
	"encoding/json"
	"sort"
	"strconv"

	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	sdkerrors "github.com/enigmampc/cosmos-sdk/types/errors"

	abci "github.com/tendermint/tendermint/abci/types"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

const (
	QueryListContractByCode = "list-contracts-by-code"
	QueryGetContract        = "contract-info"
	QueryGetContractState   = "query"
	QueryGetCode            = "code"
	QueryListCode           = "list-code"
	QueryContractHistory    = "contract-history"
	QueryContractAddress    = "label"
	QueryContractKey        = "contract-key"
	QueryContractHash       = "contract-hash"
)

const (
	QueryMethodContractStateSmart = "smart"
	QueryMethodContractStateAll   = "all"
	QueryMethodContractStateRaw   = "raw"
)

// ContractInfoWithAddress adds the address (key) to the ContractInfo representation
type ContractInfoWithAddress struct {
	// embedded here, so all json items remain top level
	*types.ContractInfo
	Address sdk.AccAddress `json:"address"`
}

// NewQuerier creates a new querier
func NewQuerier(keeper Keeper) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) ([]byte, error) {
		switch path[0] {
		case QueryGetContract:
			return queryContractInfo(ctx, path[1], keeper)
		case QueryListContractByCode:
			return queryContractListByCode(ctx, path[1], keeper)
		case QueryGetContractState:
			return queryContractState(ctx, path[1], req, keeper)
		case QueryGetCode:
			return queryCode(ctx, path[1], keeper)
		case QueryListCode:
			return queryCodeList(ctx, keeper)
		case QueryContractHistory:
			return queryContractHistory(ctx, path[1], keeper)
		case QueryContractAddress:
			return queryContractAddress(ctx, path[1], req, keeper)
		case QueryContractKey:
			return queryContractKey(ctx, path[1], req, keeper)
		case QueryContractHash:
			return queryContractHash(ctx, path[1], req, keeper)
		default:
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "unknown data query endpoint")
		}
	}
}

func queryContractInfo(ctx sdk.Context, bech string, keeper Keeper) ([]byte, error) {
	addr, err := sdk.AccAddressFromBech32(bech)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
	}
	info := keeper.GetContractInfo(ctx, addr)
	if info == nil {
		return []byte("null"), nil
	}
	redact(info)
	infoWithAddress := ContractInfoWithAddress{
		Address:      addr,
		ContractInfo: info,
	}
	bz, err := json.MarshalIndent(infoWithAddress, "", "  ")
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}
	return bz, nil
}

// redact clears all fields not in the public api
func redact(info *types.ContractInfo) {
	info.Created = nil
}

func queryContractListByCode(ctx sdk.Context, codeIDstr string, keeper Keeper) ([]byte, error) {
	codeID, err := strconv.ParseUint(codeIDstr, 10, 64)
	if err != nil {
		return nil, err
	}

	var contracts []ContractInfoWithAddress
	keeper.IterateContractInfo(ctx, func(addr sdk.AccAddress, info types.ContractInfo) bool {
		if info.CodeID == codeID {
			// and add the address
			infoWithAddress := ContractInfoWithAddress{
				Address:      addr,
				ContractInfo: &info,
			}
			contracts = append(contracts, infoWithAddress)
		}
		return false
	})

	// now we sort them by AbsoluteTxPosition
	sort.Slice(contracts, func(i, j int) bool {
		return contracts[i].ContractInfo.Created.LessThan(contracts[j].ContractInfo.Created)
	})
	// and remove that info for the final json (yes, the json:"-" tag doesn't work)
	for i := range contracts {
		redact(contracts[i].ContractInfo)
	}

	bz, err := json.MarshalIndent(contracts, "", "  ")
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}
	return bz, nil
}

func queryContractState(ctx sdk.Context, bech string, req abci.RequestQuery, keeper Keeper) ([]byte, error) {
	contractAddr, err := sdk.AccAddressFromBech32(bech)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, bech)
	}

	return keeper.QuerySmart(ctx, contractAddr, req.Data, false)
}

type GetCodeResponse struct {
	ListCodeResponse
	// Data is the entire wasm bytecode
	Data []byte `json:"data" yaml:"data"`
}

func queryCode(ctx sdk.Context, codeIDstr string, keeper Keeper) ([]byte, error) {
	codeID, err := strconv.ParseUint(codeIDstr, 10, 64)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "invalid codeID: "+err.Error())
	}

	res := keeper.GetCodeInfo(ctx, codeID)
	if res == nil {
		// nil, nil leads to 404 in rest handler
		return nil, nil
	}
	info := ListCodeResponse{
		ID:       codeID,
		Creator:  res.Creator,
		DataHash: res.CodeHash,
		Source:   res.Source,
		Builder:  res.Builder,
	}

	code, err := keeper.GetByteCode(ctx, codeID)
	if err != nil {
		return nil, sdkerrors.Wrap(err, "loading wasm code")
	}

	bz, err := json.MarshalIndent(GetCodeResponse{info, code}, "", "  ")
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}
	return bz, nil
}

type ListCodeResponse struct {
	ID       uint64           `json:"id"`
	Creator  sdk.AccAddress   `json:"creator"`
	DataHash tmbytes.HexBytes `json:"data_hash"`
	Source   string           `json:"source"`
	Builder  string           `json:"builder"`
}

func queryCodeList(ctx sdk.Context, keeper Keeper) ([]byte, error) {
	var info []ListCodeResponse

	var i uint64
	for true {
		i++
		res := keeper.GetCodeInfo(ctx, i)
		if res == nil {
			break
		}
		info = append(info, ListCodeResponse{
			ID:       i,
			Creator:  res.Creator,
			DataHash: res.CodeHash,
			Source:   res.Source,
			Builder:  res.Builder,
		})
	}

	bz, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}
	return bz, nil
}

func queryContractHistory(ctx sdk.Context, bech string, keeper Keeper) ([]byte, error) {
	contractAddr, err := sdk.AccAddressFromBech32(bech)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
	}
	entries := keeper.GetContractHistory(ctx, contractAddr)
	if entries == nil {
		// nil, nil leads to 404 in rest handler
		return nil, nil
	}
	// redact response
	for i := range entries {
		entries[i].Updated = nil
	}

	bz, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}
	return bz, nil
}

func queryContractAddress(ctx sdk.Context, label string, req abci.RequestQuery, keeper Keeper) ([]byte, error) {
	res := keeper.GetContractAddress(ctx, label)
	if res == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownAddress, label)
	}

	return res, nil
}

func queryContractKey(ctx sdk.Context, address string, req abci.RequestQuery, keeper Keeper) ([]byte, error) {
	contractAddr, err := sdk.AccAddressFromBech32(address)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, address)
	}

	res := keeper.GetContractKey(ctx, contractAddr)
	if res == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownAddress, address)
	}

	return res, nil
}

func queryContractHash(ctx sdk.Context, address string, req abci.RequestQuery, keeper Keeper) ([]byte, error) {
	contractAddr, err := sdk.AccAddressFromBech32(address)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, address)
	}

	res := keeper.GetContractInfo(ctx, contractAddr)
	if res == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownAddress, address)
	}

	return keeper.GetCodeInfo(ctx, res.CodeID).CodeHash, nil
}
