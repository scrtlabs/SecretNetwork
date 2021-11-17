package keeper

import (
	"context"
	"sort"

	"github.com/golang/protobuf/ptypes/empty"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

type GrpcQuerier struct {
	keeper Keeper
}

// todo: this needs proper tests and doc
func NewQuerier(keeper Keeper) GrpcQuerier {
	return GrpcQuerier{keeper: keeper}
}

func (q GrpcQuerier) ContractInfo(c context.Context, req *types.QueryContractInfoRequest) (*types.QueryContractInfoResponse, error) {
	if err := sdk.VerifyAddressFormat(req.Address); err != nil {
		return nil, err
	}
	rsp, err := queryContractInfo(sdk.UnwrapSDKContext(c), req.Address, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QueryContractInfoResponse{
		Address:      rsp.Address,
		ContractInfo: rsp.ContractInfo,
	}, nil
}

/*
func (q GrpcQuerier) ContractHistory(c context.Context, req *types.QueryContractHistoryRequest) (*types.QueryContractHistoryResponse, error) {
	if err := sdk.VerifyAddressFormat(req.Address); err != nil {
		return nil, err
	}
	rsp, err := queryContractHistory(sdk.UnwrapSDKContext(c), req.Address, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QueryContractHistoryResponse{
		Entries: rsp,
	}, nil
}
*/

func (q GrpcQuerier) ContractsByCode(c context.Context, req *types.QueryContractsByCodeRequest) (*types.QueryContractsByCodeResponse, error) {
	if req.CodeId == 0 {
		return nil, sdkerrors.Wrap(types.ErrInvalid, "code id")
	}
	rsp, err := queryContractListByCode(sdk.UnwrapSDKContext(c), req.CodeId, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QueryContractsByCodeResponse{
		ContractInfos: rsp,
	}, nil
}

/*
func (q GrpcQuerier) AllContractState(c context.Context, req *types.QueryAllContractStateRequest) (*types.QueryAllContractStateResponse, error) {
	if err := sdk.VerifyAddressFormat(req.Address); err != nil {
		return nil, err
	}
	ctx := sdk.UnwrapSDKContext(c)
	if !q.keeper.containsContractInfo(ctx, req.Address) {
		return nil, types.ErrNotFound
	}
	var resultData []types.Model
	for iter := q.keeper.GetContractState(ctx, req.Address); iter.Valid(); iter.Next() {
		resultData = append(resultData, types.Model{
			Key:   iter.Key(),
			Value: iter.Value(),
		})
	}
	return &types.QueryAllContractStateResponse{Models: resultData}, nil
}

func (q GrpcQuerier) RawContractState(c context.Context, req *types.QueryRawContractStateRequest) (*types.QueryRawContractStateResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)

	if err := sdk.VerifyAddressFormat(req.Address); err != nil {
		return nil, err
	}

	if !q.keeper.containsContractInfo(ctx, req.Address) {
		return nil, types.ErrNotFound
	}
	rsp := q.keeper.QueryRaw(ctx, req.Address, req.QueryData)
	return &types.QueryRawContractStateResponse{Data: rsp}, nil
}
*/

func (q GrpcQuerier) SmartContractState(c context.Context, req *types.QuerySmartContractStateRequest) (*types.QuerySmartContractStateResponse, error) {
	if err := sdk.VerifyAddressFormat(req.Address); err != nil {
		return nil, err
	}
	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(sdk.NewGasMeter(q.keeper.queryGasLimit))
	rsp, err := q.keeper.QuerySmart(ctx, req.Address, req.QueryData, false)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QuerySmartContractStateResponse{Data: rsp}, nil

}

func (q GrpcQuerier) Code(c context.Context, req *types.QueryCodeRequest) (*types.QueryCodeResponse, error) {
	if req.CodeId == 0 {
		return nil, sdkerrors.Wrap(types.ErrInvalid, "code id")
	}
	rsp, err := queryCode(sdk.UnwrapSDKContext(c), req.CodeId, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QueryCodeResponse{
		CodeInfoResponse: rsp.CodeInfoResponse,
		Data:             rsp.Data,
	}, nil
}

func (q GrpcQuerier) Codes(c context.Context, _ *empty.Empty) (*types.QueryCodesResponse, error) {
	rsp, err := queryCodeList(sdk.UnwrapSDKContext(c), q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QueryCodesResponse{CodeInfos: rsp}, nil
}

func (q GrpcQuerier) AddressByLabel(c context.Context, req *types.QueryContractAddressByLabelRequest) (*types.QueryContractAddressByLabelResponse, error) {
	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(sdk.NewGasMeter(q.keeper.queryGasLimit))
	rsp, err := queryContractAddress(ctx, req.Label, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QueryContractAddressByLabelResponse{Address: rsp}, nil

}

func (q GrpcQuerier) ContractKey(c context.Context, req *types.QueryContractKeyRequest) (*types.QueryContractKeyResponse, error) {
	if err := sdk.VerifyAddressFormat(req.Address); err != nil {
		return nil, err
	}
	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(sdk.NewGasMeter(q.keeper.queryGasLimit))
	rsp, err := queryContractKey(ctx, req.Address, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QueryContractKeyResponse{Key: rsp}, nil

}

func (q GrpcQuerier) ContractHash(c context.Context, req *types.QueryContractHashRequest) (*types.QueryContractHashResponse, error) {
	if err := sdk.VerifyAddressFormat(req.Address); err != nil {
		return nil, err
	}
	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(sdk.NewGasMeter(q.keeper.queryGasLimit))
	rsp, err := queryContractHash(ctx, req.Address, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QueryContractHashResponse{CodeHash: rsp}, nil

}

func queryContractInfo(ctx sdk.Context, addr sdk.AccAddress, keeper Keeper) (*types.ContractInfoWithAddress, error) {
	info := keeper.GetContractInfo(ctx, addr)
	if info == nil {
		return nil, nil
	}
	// redact the Created field (just used for sorting, not part of public API)
	info.Created = nil
	return &types.ContractInfoWithAddress{
		Address:      addr,
		ContractInfo: info,
	}, nil
}

func queryContractListByCode(ctx sdk.Context, codeID uint64, keeper Keeper) ([]types.ContractInfoWithAddress, error) {
	var contracts []types.ContractInfoWithAddress
	keeper.IterateContractInfo(ctx, func(addr sdk.AccAddress, info types.ContractInfo, _ types.ContractCustomInfo) bool {
		if info.CodeID == codeID {
			// and add the address
			infoWithAddress := types.ContractInfoWithAddress{
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
		contracts[i].Created = nil
	}

	return contracts, nil
}

func queryCode(ctx sdk.Context, codeID uint64, keeper Keeper) (*types.QueryCodeResponse, error) {
	if codeID == 0 {
		return nil, nil
	}
	res := keeper.GetCodeInfo(ctx, codeID)
	if res == nil {
		// nil, nil leads to 404 in rest handler
		return nil, nil
	}
	info := types.CodeInfoResponse{
		CodeID:   codeID,
		Creator:  res.Creator,
		DataHash: res.CodeHash,
		Source:   res.Source,
		Builder:  res.Builder,
	}

	code, err := keeper.GetByteCode(ctx, codeID)
	if err != nil {
		return nil, sdkerrors.Wrap(err, "loading wasm code")
	}

	return &types.QueryCodeResponse{CodeInfoResponse: &info, Data: code}, nil
}

func queryCodeList(ctx sdk.Context, keeper Keeper) ([]types.CodeInfoResponse, error) {
	var info []types.CodeInfoResponse
	keeper.IterateCodeInfos(ctx, func(i uint64, res types.CodeInfo) bool {
		info = append(info, types.CodeInfoResponse{
			CodeID:   i,
			Creator:  res.Creator,
			DataHash: res.CodeHash,
			Source:   res.Source,
			Builder:  res.Builder,
		})
		return false
	})
	return info, nil
}

/*
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
*/

func queryContractAddress(ctx sdk.Context, label string, keeper Keeper) (sdk.AccAddress, error) {
	res := keeper.GetContractAddress(ctx, label)
	if res == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownAddress, label)
	}

	return res, nil
}

func queryContractKey(ctx sdk.Context, address sdk.AccAddress, keeper Keeper) ([]byte, error) {
	res := keeper.GetContractKey(ctx, address)
	if res == nil {
		return nil, nil
	}

	return res, nil
}

func queryContractHash(ctx sdk.Context, address sdk.AccAddress, keeper Keeper) ([]byte, error) {
	res := keeper.GetContractInfo(ctx, address)
	if res == nil {
		return nil, nil
	}

	return keeper.GetCodeInfo(ctx, res.CodeID).CodeHash, nil
}
