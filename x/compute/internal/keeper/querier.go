package keeper

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
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

func (q GrpcQuerier) Params(c context.Context, _ *types.ParamsRequest) (*types.ParamsResponse, error) {
	params := q.keeper.GetParams(sdk.UnwrapSDKContext(c))
	return &types.ParamsResponse{
		Params: params,
	}, nil
}

// AuthorizedMigration returns the authorized migration info for a contract
func (q GrpcQuerier) AuthorizedMigration(c context.Context, req *types.QueryAuthorizedMigrationRequest) (*types.QueryAuthorizedMigrationResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.ContractAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "contract address cannot be empty")
	}

	ctx := sdk.UnwrapSDKContext(c)

	// Validate contract address
	if _, err := sdk.AccAddressFromBech32(req.ContractAddress); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid contract address")
	}

	// Check for authorized migration
	codeID, hasAuth := q.keeper.GetAuthorizedMigration(ctx, req.ContractAddress)

	response := &types.QueryAuthorizedMigrationResponse{}
	if hasAuth {
		response.NewCodeID = codeID
	} else {
		return nil, status.Error(codes.NotFound, "no authorized migration found for the given contract address")
	}

	return response, nil
}

// AuthorizedAdminUpdate returns the authorized admin update info for a contract
func (q GrpcQuerier) AuthorizedAdminUpdate(c context.Context, req *types.QueryAuthorizedAdminUpdateRequest) (*types.QueryAuthorizedAdminUpdateResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.ContractAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "contract address cannot be empty")
	}

	ctx := sdk.UnwrapSDKContext(c)

	// Validate contract address
	if _, err := sdk.AccAddressFromBech32(req.ContractAddress); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid contract address")
	}

	// Check for authorized admin update
	newAdmin, hasAuth := q.keeper.GetNewAdmin(ctx, req.ContractAddress)

	response := &types.QueryAuthorizedAdminUpdateResponse{}
	if hasAuth {
		response.NewAdmin = newAdmin
	} else {
		return nil, status.Error(codes.NotFound, "no authorized admin update found for the given contract address")
	}

	return response, nil
}

// EcallRecord returns the ecall record for a specific block height
// This is used by non-SGX nodes to sync with the network
func (q GrpcQuerier) EcallRecord(c context.Context, req *types.QueryEcallRecordRequest) (*types.QueryEcallRecordResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.Height <= 0 {
		return nil, status.Error(codes.InvalidArgument, "height must be positive")
	}

	recorder := api.GetRecorder()
	random, evidence, found := recorder.ReplaySubmitBlockSignatures(req.Height)
	if !found {
		return nil, status.Error(codes.NotFound, "no ecall record found for the given height")
	}

	return &types.QueryEcallRecordResponse{
		Height:               req.Height,
		RandomSeed:           random,
		ValidatorSetEvidence: evidence,
	}, nil
}

// EcallRecords returns ecall records for a range of block heights
// This is used by non-SGX nodes to batch sync with the network
func (q GrpcQuerier) EcallRecords(c context.Context, req *types.QueryEcallRecordsRequest) (*types.QueryEcallRecordsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.StartHeight <= 0 {
		return nil, status.Error(codes.InvalidArgument, "start_height must be positive")
	}

	if req.EndHeight < req.StartHeight {
		return nil, status.Error(codes.InvalidArgument, "end_height must be >= start_height")
	}

	// Limit the range to prevent abuse (max 1000 blocks per request)
	maxRange := int64(1000)
	if req.EndHeight-req.StartHeight > maxRange {
		return nil, status.Errorf(codes.InvalidArgument, "range too large, max %d blocks per request", maxRange)
	}

	recorder := api.GetRecorder()
	var records []types.QueryEcallRecordResponse

	for height := req.StartHeight; height <= req.EndHeight; height++ {
		random, evidence, found := recorder.ReplaySubmitBlockSignatures(height)
		if found {
			records = append(records, types.QueryEcallRecordResponse{
				Height:               height,
				RandomSeed:           random,
				ValidatorSetEvidence: evidence,
			})
		}
	}

	return &types.QueryEcallRecordsResponse{
		Records: records,
	}, nil
}

// EncryptedSeed returns the encrypted seed for a specific certificate hash
// This is used by non-SGX nodes to sync with the network
func (q GrpcQuerier) EncryptedSeed(c context.Context, req *types.QueryEncryptedSeedRequest) (*types.QueryEncryptedSeedResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.CertHash == "" {
		return nil, status.Error(codes.InvalidArgument, "cert_hash is required")
	}

	// Decode hex string to bytes
	certHash, err := hex.DecodeString(req.CertHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid cert_hash: must be hex encoded")
	}

	recorder := api.GetRecorder()
	encryptedSeed, found := recorder.ReplayGetEncryptedSeed(certHash)
	if !found {
		return nil, status.Error(codes.NotFound, "no encrypted seed found for the given certificate hash")
	}

	return &types.QueryEncryptedSeedResponse{
		EncryptedSeed: encryptedSeed,
	}, nil
}

// BlockTraces returns all execution traces for a specific block height
// This is used by non-SGX nodes to batch fetch all traces for a block
func (q GrpcQuerier) BlockTraces(c context.Context, req *types.QueryBlockTracesRequest) (*types.QueryBlockTracesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.Height <= 0 {
		return nil, status.Error(codes.InvalidArgument, "height must be positive")
	}

	fmt.Printf("[BlockTraces] DEBUG: Query received for height=%d\n", req.Height)
	recorder := api.GetRecorder()
	traces, err := recorder.GetAllTracesForBlock(req.Height)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get traces: %v", err)
	}

	fmt.Printf("[BlockTraces] DEBUG: Retrieved %d traces from database\n", len(traces))

	// Convert api.ExecutionTrace to types.ExecutionTraceData
	protoTraces := make([]types.ExecutionTraceData, len(traces))
	for i, trace := range traces {
		fmt.Printf("[BlockTraces] DEBUG: Converting trace index=%d callbackGas=%d\n", trace.Index, trace.CallbackGas)
		ops := make([]types.StorageOp, len(trace.Ops))
		for j, op := range trace.Ops {
			ops[j] = types.StorageOp{
				IsDelete: op.IsDelete,
				Key:      op.Key,
				Value:    op.Value,
			}
		}
		protoTraces[i] = types.ExecutionTraceData{
			Index:       trace.Index,
			Ops:         ops,
			Result:      trace.Result,
			GasUsed:     trace.GasUsed,
			CallbackGas: trace.CallbackGas,
			HasError:    trace.HasError,
			ErrorMsg:    trace.ErrorMsg,
		}
		fmt.Printf("[BlockTraces] DEBUG: Proto trace callbackGas=%d\n", protoTraces[i].CallbackGas)
	}

	fmt.Printf("[BlockTraces] DEBUG: Returning %d traces, first trace callbackGas=%d\n", len(protoTraces),
		func() uint64 {
			if len(protoTraces) > 0 {
				return protoTraces[0].CallbackGas
			}
			return 0
		}())

	return &types.QueryBlockTracesResponse{
		Traces: protoTraces,
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
