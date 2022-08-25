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

func (q GrpcQuerier) ContractInfo(c context.Context, req *types.QueryByAddressRequest) (*types.QueryContractInfoResponse, error) {
	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, err
	}

	if err := sdk.VerifyAddressFormat(addr); err != nil {
		return nil, err
	}
	rsp, err := queryContractInfo(sdk.UnwrapSDKContext(c), addr, q.keeper)
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

func (q GrpcQuerier) ContractsByCode(c context.Context, req *types.QueryByIDRequest) (*types.QueryContractsByCodeResponse, error) {
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

func (q GrpcQuerier) SmartContractState(c context.Context, req *types.QuerySmartContractStateRequest) (*types.QuerySmartContractStateResponse, error) {
	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, err
	}

	if err := sdk.VerifyAddressFormat(addr); err != nil {
		return nil, err
	}
	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(sdk.NewGasMeter(q.keeper.queryGasLimit))
	rsp, err := q.keeper.QuerySmart(ctx, addr, req.QueryData, false)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QuerySmartContractStateResponse{Data: rsp}, nil
}

func (q GrpcQuerier) Code(c context.Context, req *types.QueryByIDRequest) (*types.QueryCodeResponse, error) {
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

func (q GrpcQuerier) LabelByAddress(c context.Context, req *types.QueryByAddressRequest) (*types.QueryContractLabelResponse, error) {
	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(sdk.NewGasMeter(q.keeper.queryGasLimit))
	rsp, err := queryContractLabel(ctx, addr, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == "":
		return nil, types.ErrNotFound
	}
	return &types.QueryContractLabelResponse{Label: rsp}, nil
}

func (q GrpcQuerier) AddressByLabel(c context.Context, req *types.QueryByLabelRequest) (*types.QueryContractAddressResponse, error) {
	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(sdk.NewGasMeter(q.keeper.queryGasLimit))
	rsp, err := queryContractAddress(ctx, req.Label, q.keeper)

	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QueryContractAddressResponse{Address: rsp.String()}, nil
}

func (q GrpcQuerier) ContractKey(c context.Context, req *types.QueryByAddressRequest) (*types.QueryContractKeyResponse, error) {
	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, err
	}

	if err := sdk.VerifyAddressFormat(addr); err != nil {
		return nil, err
	}
	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(sdk.NewGasMeter(q.keeper.queryGasLimit))
	rsp, err := queryContractKey(ctx, addr, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QueryContractKeyResponse{Key: rsp}, nil
}

func (q GrpcQuerier) ContractHash(c context.Context, req *types.QueryByAddressRequest) (*types.QueryContractHashResponse, error) {
	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, err
	}

	if err := sdk.VerifyAddressFormat(addr); err != nil {
		return nil, err
	}
	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(sdk.NewGasMeter(q.keeper.queryGasLimit))
	rsp, err := queryContractHash(ctx, addr, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case rsp == nil:
		return nil, types.ErrNotFound
	}
	return &types.QueryContractHashResponse{CodeHash: rsp}, nil
}

func (q GrpcQuerier) ContractHashByID(c context.Context, req *types.QueryByIDRequest) (*types.QueryContractHashResponse, error) {
	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(sdk.NewGasMeter(q.keeper.queryGasLimit))
	rsp, err := queryContractHashByID(ctx, req.CodeId, q.keeper)
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
		Address:      addr.String(),
		ContractInfo: info,
	}, nil
}

func queryContractListByCode(ctx sdk.Context, codeID uint64, keeper Keeper) ([]types.ContractInfoWithAddress, error) {
	var contracts []types.ContractInfoWithAddress
	keeper.IterateContractInfo(ctx, func(addr sdk.AccAddress, info types.ContractInfo, _ types.ContractCustomInfo) bool {
		if info.CodeID == codeID {
			// and add the address
			infoWithAddress := types.ContractInfoWithAddress{
				Address:      addr.String(),
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
		Creator:  res.Creator.String(),
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
			Creator:  res.Creator.String(),
			DataHash: res.CodeHash,
			Source:   res.Source,
			Builder:  res.Builder,
		})
		return false
	})
	return info, nil
}

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

	return queryContractHashByID(ctx, res.CodeID, keeper)
}

func queryContractLabel(ctx sdk.Context, address sdk.AccAddress, keeper Keeper) (string, error) {
	res := keeper.GetContractInfo(ctx, address)
	if res == nil {
		return "", nil
	}

	return res.Label, nil
}

func queryContractHashByID(ctx sdk.Context, codeID uint64, keeper Keeper) ([]byte, error) {
	codeInfo := keeper.GetCodeInfo(ctx, codeID)

	if codeInfo == nil {
		return nil, nil
	}

	return codeInfo.CodeHash, nil
}
