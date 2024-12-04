package keeper

import (
	"context"
	"encoding/hex"
	"sort"

	"github.com/golang/protobuf/ptypes/empty"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

const (
	QueryListContractByCode       = "list-contracts-by-code"
	QueryGetContract              = "contract-info"
	QueryGetContractState         = "contract-state"
	QueryGetCode                  = "code"
	QueryListCode                 = "list-code"
	QueryContractAddress          = "label"
	QueryContractKey              = "contract-key"
	QueryContractHash             = "contract-hash"
	QueryContractHashByCodeID     = "contract-hash-by-id"
	QueryMethodContractStateSmart = "smart"
)

var _ types.QueryServer = GrpcQuerier{} // type assertion

type GrpcQuerier struct {
	keeper Keeper
}

func (q GrpcQuerier) ContractHistory(c context.Context, req *types.QueryContractHistoryRequest) (*types.QueryContractHistoryResponse, error) {
	contractAddress, err := sdk.AccAddressFromBech32(req.ContractAddress)
	if err != nil {
		return nil, err
	}

	return &types.QueryContractHistoryResponse{
		Entries: q.keeper.GetContractHistory(sdk.UnwrapSDKContext(c), contractAddress),
	}, nil
}

func NewGrpcQuerier(keeper Keeper) GrpcQuerier {
	return GrpcQuerier{keeper: keeper}
}

func (q GrpcQuerier) ContractInfo(c context.Context, req *types.QueryByContractAddressRequest) (*types.QueryContractInfoResponse, error) {
	contractAddress, err := sdk.AccAddressFromBech32(req.ContractAddress)
	if err != nil {
		return nil, err
	}

	response, err := queryContractInfo(sdk.UnwrapSDKContext(c), contractAddress, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case response == nil:
		return nil, types.ErrNotFound
	}

	return &types.QueryContractInfoResponse{
		ContractAddress: response.ContractAddress,
		ContractInfo:    response.ContractInfo,
	}, nil
}

func (q GrpcQuerier) ContractsByCodeId(c context.Context, req *types.QueryByCodeIdRequest) (*types.QueryContractsByCodeIdResponse, error) {
	if req.CodeId == 0 {
		return nil, errorsmod.Wrap(types.ErrInvalid, "code id")
	}

	response, err := queryContractListByCode(sdk.UnwrapSDKContext(c), req.CodeId, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case response == nil:
		return nil, types.ErrNotFound
	}

	return &types.QueryContractsByCodeIdResponse{
		ContractInfos: response,
	}, nil
}

func (q GrpcQuerier) QuerySecretContract(c context.Context, req *types.QuerySecretContractRequest) (*types.QuerySecretContractResponse, error) {
	contractAddress, err := sdk.AccAddressFromBech32(req.ContractAddress)
	if err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(storetypes.NewGasMeter(q.keeper.queryGasLimit))

	response, err := q.keeper.QuerySmart(ctx, contractAddress, req.Query, false)
	switch {
	case err != nil:
		return nil, err
	case response == nil:
		return nil, types.ErrNotFound
	}

	return &types.QuerySecretContractResponse{Data: response}, nil
}

func (q GrpcQuerier) Code(c context.Context, req *types.QueryByCodeIdRequest) (*types.QueryCodeResponse, error) {
	if req.CodeId == 0 {
		return nil, errorsmod.Wrap(types.ErrInvalid, "code id")
	}

	response, err := queryCode(sdk.UnwrapSDKContext(c), req.CodeId, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case response == nil:
		return nil, types.ErrNotFound
	}

	return &types.QueryCodeResponse{
		CodeInfoResponse: response.CodeInfoResponse,
		Wasm:             response.Wasm,
	}, nil
}

func (q GrpcQuerier) Codes(c context.Context, _ *empty.Empty) (*types.QueryCodesResponse, error) {
	response, err := queryCodeList(sdk.UnwrapSDKContext(c), q.keeper)
	if err != nil {
		return nil, err
	}
	if response == nil {
		response = make([]types.CodeInfoResponse, 0)
	}
	return &types.QueryCodesResponse{CodeInfos: response}, nil
}

func (q GrpcQuerier) CodeHashByContractAddress(c context.Context, req *types.QueryByContractAddressRequest) (*types.QueryCodeHashResponse, error) {
	contractAddress, err := sdk.AccAddressFromBech32(req.ContractAddress)
	if err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(storetypes.NewGasMeter(q.keeper.queryGasLimit))

	codeHashBz, err := queryCodeHashByAddress(ctx, contractAddress, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case codeHashBz == nil:
		return nil, types.ErrNotFound
	}

	return &types.QueryCodeHashResponse{
		CodeHash: hex.EncodeToString(codeHashBz),
	}, nil
}

func (q GrpcQuerier) CodeHashByCodeId(c context.Context, req *types.QueryByCodeIdRequest) (*types.QueryCodeHashResponse, error) {
	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(storetypes.NewGasMeter(q.keeper.queryGasLimit))

	codeHashBz, err := queryCodeHashByCodeID(ctx, req.CodeId, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case codeHashBz == nil:
		return nil, types.ErrNotFound
	}

	return &types.QueryCodeHashResponse{
		CodeHash: hex.EncodeToString(codeHashBz),
	}, nil
}

func (q GrpcQuerier) LabelByAddress(c context.Context, req *types.QueryByContractAddressRequest) (*types.QueryContractLabelResponse, error) {
	contractAddress, err := sdk.AccAddressFromBech32(req.ContractAddress)
	if err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(storetypes.NewGasMeter(q.keeper.queryGasLimit))

	response, err := queryContractInfo(ctx, contractAddress, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case response == nil:
		return nil, types.ErrNotFound
	}

	return &types.QueryContractLabelResponse{
		Label: response.Label,
	}, nil
}

func (q GrpcQuerier) AddressByLabel(c context.Context, req *types.QueryByLabelRequest) (*types.QueryContractAddressResponse, error) {
	ctx := sdk.UnwrapSDKContext(c).WithGasMeter(storetypes.NewGasMeter(q.keeper.queryGasLimit))

	response, err := queryContractAddress(ctx, req.Label, q.keeper)
	switch {
	case err != nil:
		return nil, err
	case response == nil:
		return nil, types.ErrNotFound
	}

	return &types.QueryContractAddressResponse{
		ContractAddress: response.String(),
	}, nil
}

func (q GrpcQuerier) Params(c context.Context, req *types.ParamsRequest) (*types.ParamsResponse, error) {
	params := q.keeper.GetParams(sdk.UnwrapSDKContext(c))
	return &types.ParamsResponse{
		Params: params,
	}, nil
}

func queryContractInfo(ctx sdk.Context, contractAddress sdk.AccAddress, keeper Keeper) (*types.ContractInfoWithAddress, error) {
	info := keeper.GetContractInfo(ctx, contractAddress)
	if info == nil {
		return nil, nil
	}

	info.AdminProof = nil // for internal usage only

	return &types.ContractInfoWithAddress{
		ContractAddress: contractAddress.String(),
		ContractInfo:    info,
	}, nil
}

func queryContractListByCode(ctx sdk.Context, codeID uint64, keeper Keeper) ([]types.ContractInfoWithAddress, error) {
	var contracts []types.ContractInfoWithAddress
	keeper.IterateContractInfo(ctx, func(addr sdk.AccAddress, info types.ContractInfo, _ types.ContractCustomInfo) bool {
		if info.CodeID == codeID {
			info.AdminProof = nil // for internal usage only

			// and add the address
			infoWithAddress := types.ContractInfoWithAddress{
				ContractAddress: addr.String(),
				ContractInfo:    &info,
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

func queryCode(ctx sdk.Context, codeId uint64, keeper Keeper) (*types.QueryCodeResponse, error) {
	if codeId == 0 {
		return nil, nil
	}

	codeInfo, err := keeper.GetCodeInfo(ctx, codeId)
	if err != nil {
		return nil, nil
	}

	info := types.CodeInfoResponse{
		CodeId:   codeId,
		Creator:  codeInfo.Creator.String(),
		CodeHash: hex.EncodeToString(codeInfo.CodeHash),
		Source:   codeInfo.Source,
		Builder:  codeInfo.Builder,
	}

	wasmBz, err := keeper.GetWasm(ctx, codeId)
	if err != nil {
		return nil, errorsmod.Wrap(err, "loading wasm code")
	}

	return &types.QueryCodeResponse{
		CodeInfoResponse: &info,
		Wasm:             wasmBz,
	}, nil
}

func queryCodeList(ctx sdk.Context, keeper Keeper) ([]types.CodeInfoResponse, error) {
	var info []types.CodeInfoResponse
	keeper.IterateCodeInfos(ctx, func(codeId uint64, res types.CodeInfo) bool {
		info = append(info, types.CodeInfoResponse{
			CodeId:   codeId,
			Creator:  res.Creator.String(),
			CodeHash: hex.EncodeToString(res.CodeHash),
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
		return nil, sdkerrors.ErrUnknownAddress.Wrap(label)
	}

	return res, nil
}

func queryCodeHashByAddress(ctx sdk.Context, address sdk.AccAddress, keeper Keeper) ([]byte, error) {
	res := keeper.GetContractInfo(ctx, address)
	if res == nil {
		return nil, nil
	}

	return queryCodeHashByCodeID(ctx, res.CodeID, keeper)
}

func queryCodeHashByCodeID(ctx sdk.Context, codeID uint64, keeper Keeper) ([]byte, error) {
	codeInfo, err := keeper.GetCodeInfo(ctx, codeID)
	if err != nil {
		return nil, err
	}

	return codeInfo.CodeHash, nil
}
