package keeper

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/x/staking"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	wasm "github.com/enigmampc/EnigmaBlockchain/go-cosmwasm"
	wasmTypes "github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/types"
	"github.com/tendermint/tendermint/crypto"

	"github.com/enigmampc/EnigmaBlockchain/x/compute/internal/types"
)

// GasMultiplier is how many cosmwasm gas points = 1 sdk gas point
// SDK reference costs can be found here: https://github.com/cosmos/cosmos-sdk/blob/02c6c9fafd58da88550ab4d7d494724a477c8a68/store/types/gas.go#L153-L164
// A write at ~3000 gas and ~200us = 10 gas per us (microsecond) cpu/io
// Rough timing have 88k gas at 90us, which is equal to 1k sdk gas... (one read)
const GasMultiplier = 100

// MaxGas for a contract is 900 million (enforced in rust)
const MaxGas = 900_000_000

// Keeper will have a reference to Wasmer with it's own data directory.
type Keeper struct {
	storeKey      sdk.StoreKey
	cdc           *codec.Codec
	accountKeeper auth.AccountKeeper
	bankKeeper    bank.Keeper

	wasmer       wasm.Wasmer
	queryPlugins QueryPlugins
	messenger    MessageHandler
	// queryGasLimit is the max wasm gas that can be spent on executing a query with a contract
	queryGasLimit uint64
}

// NewKeeper creates a new contract Keeper instance
// If customEncoders is non-nil, we can use this to override some of the message handler, especially custom
func NewKeeper(cdc *codec.Codec, storeKey sdk.StoreKey, accountKeeper auth.AccountKeeper, bankKeeper bank.Keeper,
	stakingKeeper staking.Keeper,
	router sdk.Router, homeDir string, wasmConfig types.WasmConfig, supportedFeatures string, customEncoders *MessageEncoders, customPlugins *QueryPlugins) Keeper {
	wasmer, err := wasm.NewWasmer(filepath.Join(homeDir, "wasm"), supportedFeatures, wasmConfig.CacheSize)
	if err != nil {
		panic(err)
	}

	messenger := NewMessageHandler(router, customEncoders)

	keeper := Keeper{
		storeKey:      storeKey,
		cdc:           cdc,
		wasmer:        *wasmer,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
		messenger:     messenger,
		queryGasLimit: wasmConfig.SmartQueryGasLimit,
	}
	keeper.queryPlugins = DefaultQueryPlugins(bankKeeper, stakingKeeper, keeper).Merge(customPlugins)
	return keeper
}

// Create uploads and compiles a WASM contract, returning a short identifier for the contract
func (k Keeper) Create(ctx sdk.Context, creator sdk.AccAddress, wasmCode []byte, source string, builder string) (codeID uint64, err error) {
	wasmCode, err = uncompress(wasmCode)
	if err != nil {
		return 0, sdkerrors.Wrap(types.ErrCreateFailed, err.Error())
	}
	codeHash, err := k.wasmer.Create(wasmCode)
	if err != nil {
		// return 0, sdkerrors.Wrap(err, "cosmwasm create")
		return 0, sdkerrors.Wrap(types.ErrCreateFailed, err.Error())
	}
	store := ctx.KVStore(k.storeKey)
	codeID = k.autoIncrementID(ctx, types.KeyLastCodeID)
	codeInfo := types.NewCodeInfo(codeHash, creator, source, builder)
	// 0x01 | codeID (uint64) -> ContractInfo
	store.Set(types.GetCodeKey(codeID), k.cdc.MustMarshalBinaryBare(codeInfo))

	return codeID, nil
}

// Instantiate creates an instance of a WASM contract
func (k Keeper) Instantiate(ctx sdk.Context, codeID uint64, creator, admin sdk.AccAddress, initMsg []byte, label string, deposit sdk.Coins) (sdk.AccAddress, error) {
	// create contract address
	contractAddress := k.generateContractAddress(ctx, codeID)
	existingAcct := k.accountKeeper.GetAccount(ctx, contractAddress)
	if existingAcct != nil {
		return nil, sdkerrors.Wrap(types.ErrAccountExists, existingAcct.GetAddress().String())
	}

	// deposit initial contract funds
	if !deposit.IsZero() {
		sdkerr := k.bankKeeper.SendCoins(ctx, creator, contractAddress, deposit)
		if sdkerr != nil {
			return nil, sdkerr
		}
	} else {
		// create an empty account (so we don't have issues later)
		// TODO: can we remove this?
		contractAccount := k.accountKeeper.NewAccountWithAddress(ctx, contractAddress)
		k.accountKeeper.SetAccount(ctx, contractAccount)
	}

	// get contact info
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.GetCodeKey(codeID))
	if bz == nil {
		return nil, sdkerrors.Wrap(types.ErrNotFound, "contract")
	}
	var codeInfo types.CodeInfo
	k.cdc.MustUnmarshalBinaryBare(bz, &codeInfo)

	// prepare params for contract instantiate call
	params := types.NewEnv(ctx, creator, deposit, contractAddress, nil)

	// create prefixed data store
	// 0x03 | contractAddress (sdk.AccAddress)
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), prefixStoreKey)

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
	}

	// instantiate wasm contract
	gas := gasForContract(ctx)
	res, key, gasUsed, err := k.wasmer.Instantiate(codeInfo.CodeHash, params, initMsg, prefixStore, cosmwasmAPI, querier, ctx.GasMeter(), gas)
	consumeGas(ctx, gasUsed)
	if err != nil {
		return contractAddress, sdkerrors.Wrap(types.ErrInstantiateFailed, err.Error())
	}

	// emit all events from this contract itself
	value := types.CosmosResult(*res, contractAddress)
	ctx.EventManager().EmitEvents(value.Events)

	err = k.dispatchMessages(ctx, contractAddress, res.Messages)
	if err != nil {
		return nil, err
	}

	// persist instance
	createdAt := types.NewCreatedAt(ctx)
	instance := types.NewContractInfo(codeID, creator, admin, initMsg, label, createdAt)
	store.Set(types.GetContractAddressKey(contractAddress), k.cdc.MustMarshalBinaryBare(instance))

	fmt.Printf("Storing key: %s for account %s", key, contractAddress)

	store.Set(types.GetContractEnclaveKey(contractAddress), key)

	return contractAddress, nil
}

// Execute executes the contract instance
func (k Keeper) Execute(ctx sdk.Context, contractAddress sdk.AccAddress, caller sdk.AccAddress, msg []byte, coins sdk.Coins) (sdk.Result, error) {
	codeInfo, prefixStore, err := k.contractInstance(ctx, contractAddress)
	if err != nil {
		return sdk.Result{}, err
	}

	// add more funds
	if !coins.IsZero() {
		sdkerr := k.bankKeeper.SendCoins(ctx, caller, contractAddress, coins)
		if sdkerr != nil {
			return sdk.Result{}, sdkerr
		}
	}

	contractKey := store.Get(types.GetContractEnclaveKey(contractAddress))
	fmt.Printf("Contract Execute: Got contract Key for contract %s: %s\n", contractAddress, base64.StdEncoding.EncodeToString(contractKey))
	params := types.NewEnv(ctx, caller, coins, contractAddress)
	fmt.Printf("Contract Execute: key from params %s \n", params.Key)

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
	}

	gas := gasForContract(ctx)
	res, gasUsed, execErr := k.wasmer.Execute(codeInfo.CodeHash, params, msg, prefixStore, cosmwasmAPI, querier, ctx.GasMeter(), gas)
	consumeGas(ctx, gasUsed)

	if execErr != nil {
		return sdk.Result{}, sdkerrors.Wrap(types.ErrExecuteFailed, execErr.Error())
	}

	consumeGas(ctx, gasUsed)
	var result wasmTypes.CosmosResponse
	err = json.Unmarshal(res, &result)
	if err != nil {
		return sdk.Result{}, err
	}

	// emit all events from this contract itself
	value := types.CosmosResult(result.Ok, contractAddress)
	ctx.EventManager().EmitEvents(value.Events)

	// TODO: capture events here as well
	err = k.dispatchMessages(ctx, contractAccount, result.Ok.Messages)
	if err != nil {
		return sdk.Result{}, err
	}

	return sdk.Result{
		Data: res,
	}, nil
}

// Migrate allows to upgrade a contract to a new code with data migration.
func (k Keeper) Migrate(ctx sdk.Context, contractAddress sdk.AccAddress, caller sdk.AccAddress, newCodeID uint64, msg []byte) (*sdk.Result, error) {
	contractInfo := k.GetContractInfo(ctx, contractAddress)
	if contractInfo == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "unknown contract")
	}
	if contractInfo.Admin == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "migration not supported by this contract")
	}
	if !contractInfo.Admin.Equals(caller) {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "no permission")
	}
	newCodeInfo := k.GetCodeInfo(ctx, newCodeID)
	if newCodeInfo == nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "unknown code")
	}

	var noDeposit sdk.Coins
	params := types.NewEnv(ctx, caller, noDeposit, contractAddress)

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
	}

	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), prefixStoreKey)
	gas := gasForContract(ctx)
	res, gasUsed, err := k.wasmer.Migrate(newCodeInfo.CodeHash, params, msg, &prefixStore, cosmwasmAPI, &querier, ctx.GasMeter(), gas)
	consumeGas(ctx, gasUsed)
	if err != nil {
		return nil, sdkerrors.Wrap(types.ErrMigrationFailed, err.Error())
	}

	// emit all events from this contract migration itself
	value := types.CosmosResult(*res, contractAddress)
	ctx.EventManager().EmitEvents(value.Events)
	value.Events = nil

	contractInfo.UpdateCodeID(ctx, newCodeID)
	k.setContractInfo(ctx, contractAddress, contractInfo)

	if err := k.dispatchMessages(ctx, contractAddress, res.Messages); err != nil {
		return nil, sdkerrors.Wrap(err, "dispatch")
	}

	return &value, nil
}

// UpdateContractAdmin sets the admin value on the ContractInfo. New admin can be nil to disable further migrations/ updates.
func (k Keeper) UpdateContractAdmin(ctx sdk.Context, contractAddress sdk.AccAddress, caller sdk.AccAddress, newAdmin sdk.AccAddress) error {
	contractInfo := k.GetContractInfo(ctx, contractAddress)
	if contractInfo == nil {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "unknown contract")
	}
	if contractInfo.Admin == nil {
		return sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "migration not supported by this contract")
	}
	if !contractInfo.Admin.Equals(caller) {
		return sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "no permission")
	}
	contractInfo.Admin = newAdmin
	k.setContractInfo(ctx, contractAddress, contractInfo)
	return nil
}

// QuerySmart queries the smart contract itself.
func (k Keeper) QuerySmart(ctx sdk.Context, contractAddr sdk.AccAddress, req []byte) ([]byte, error) {
	ctx = ctx.WithGasMeter(sdk.NewGasMeter(k.queryGasLimit))

	codeInfo, prefixStore, err := k.contractInstance(ctx, contractAddr)
	if err != nil {
		return nil, err
	}

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
	}

	store := ctx.KVStore(k.storeKey)
	// 0x01 | codeID (uint64) -> ContractInfo
	contractKey := store.Get(types.GetContractEnclaveKey(contractAddr))

	queryResult, gasUsed, qErr := k.wasmer.Query(codeInfo.CodeHash, append(contractKey[:], req[:]...), prefixStore, cosmwasmAPI, querier, ctx.GasMeter(), gasForContract(ctx))
	consumeGas(ctx, gasUsed)
	if qErr != nil {
		return nil, sdkerrors.Wrap(types.ErrQueryFailed, qErr.Error())
	}
	return queryResult, nil
}

// QueryRaw returns the contract's state for give key. For a `nil` key a empty slice result is returned.
func (k Keeper) QueryRaw(ctx sdk.Context, contractAddress sdk.AccAddress, key []byte) []types.Model {
	result := make([]types.Model, 0)
	if key == nil {
		return result
	}
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), prefixStoreKey)

	if val := prefixStore.Get(key); val != nil {
		return append(result, types.Model{
			Key:   key,
			Value: val,
		})
	}
	return result
}

func (k Keeper) contractInstance(ctx sdk.Context, contractAddress sdk.AccAddress) (types.CodeInfo, prefix.Store, error) {
	store := ctx.KVStore(k.storeKey)

	contractBz := store.Get(types.GetContractAddressKey(contractAddress))
	if contractBz == nil {
		return types.CodeInfo{}, prefix.Store{}, sdkerrors.Wrap(types.ErrNotFound, "contract")
	}
	var contract types.ContractInfo
	k.cdc.MustUnmarshalBinaryBare(contractBz, &contract)

	contractInfoBz := store.Get(types.GetCodeKey(contract.CodeID))
	if contractInfoBz == nil {
		return types.CodeInfo{}, prefix.Store{}, sdkerrors.Wrap(types.ErrNotFound, "contract info")
	}
	var codeInfo types.CodeInfo
	k.cdc.MustUnmarshalBinaryBare(contractInfoBz, &codeInfo)
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), prefixStoreKey)
	return codeInfo, prefixStore, nil
}

func (k Keeper) GetContractInfo(ctx sdk.Context, contractAddress sdk.AccAddress) *types.ContractInfo {
	store := ctx.KVStore(k.storeKey)
	var contract types.ContractInfo
	contractBz := store.Get(types.GetContractAddressKey(contractAddress))
	if contractBz == nil {
		return nil
	}
	k.cdc.MustUnmarshalBinaryBare(contractBz, &contract)
	return &contract
}

func (k Keeper) setContractInfo(ctx sdk.Context, contractAddress sdk.AccAddress, contract *types.ContractInfo) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetContractAddressKey(contractAddress), k.cdc.MustMarshalBinaryBare(contract))
}

func (k Keeper) ListContractInfo(ctx sdk.Context, cb func(sdk.AccAddress, types.ContractInfo) bool) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.ContractKeyPrefix)
	iter := prefixStore.Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		var contract types.ContractInfo
		k.cdc.MustUnmarshalBinaryBare(iter.Value(), &contract)
		// cb returns true to stop early
		if cb(iter.Key(), contract) {
			break
		}
	}
}

func (k Keeper) GetContractState(ctx sdk.Context, contractAddress sdk.AccAddress) sdk.Iterator {
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), prefixStoreKey)
	return prefixStore.Iterator(nil, nil)
}

func (k Keeper) setContractState(ctx sdk.Context, contractAddress sdk.AccAddress, models []types.Model) {
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), prefixStoreKey)
	for _, model := range models {
		if model.Value == nil {
			model.Value = []byte{}
		}
		prefixStore.Set(model.Key, model.Value)
	}
}

func (k Keeper) GetCodeInfo(ctx sdk.Context, codeID uint64) *types.CodeInfo {
	store := ctx.KVStore(k.storeKey)
	var codeInfo types.CodeInfo
	codeInfoBz := store.Get(types.GetCodeKey(codeID))
	if codeInfoBz == nil {
		return nil
	}
	k.cdc.MustUnmarshalBinaryBare(codeInfoBz, &codeInfo)
	return &codeInfo
}

func (k Keeper) GetByteCode(ctx sdk.Context, codeID uint64) ([]byte, error) {
	store := ctx.KVStore(k.storeKey)
	var codeInfo types.CodeInfo
	codeInfoBz := store.Get(types.GetCodeKey(codeID))
	if codeInfoBz == nil {
		return nil, nil
	}
	k.cdc.MustUnmarshalBinaryBare(codeInfoBz, &codeInfo)
	return k.wasmer.GetCode(codeInfo.CodeHash)
}

func (k Keeper) dispatchMessages(ctx sdk.Context, contractAddr sdk.AccAddress, msgs []wasmTypes.CosmosMsg) error {
	for _, msg := range msgs {
		if err := k.messenger.Dispatch(ctx, contractAddr, msg); err != nil {
			return err
		}
	}
	return nil
}

func gasForContract(ctx sdk.Context) uint64 {
	meter := ctx.GasMeter()
	remaining := (meter.Limit() - meter.GasConsumed()) * GasMultiplier
	if remaining > MaxGas {
		return MaxGas
	}
	return remaining
}

func consumeGas(ctx sdk.Context, gas uint64) {
	consumed := gas / GasMultiplier
	ctx.GasMeter().ConsumeGas(consumed, "wasm contract")
}

// generates a contract address from codeID + instanceID
func (k Keeper) generateContractAddress(ctx sdk.Context, codeID uint64) sdk.AccAddress {
	instanceID := k.autoIncrementID(ctx, types.KeyLastInstanceID)
	// NOTE: It is possible to get a duplicate address if either codeID or instanceID
	// overflow 32 bits. This is highly improbable, but something that could be refactored.
	contractID := codeID<<32 + instanceID
	return addrFromUint64(contractID)
}

func (k Keeper) GetNextCodeID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.KeyLastCodeID)
	id := uint64(1)
	if bz != nil {
		id = binary.BigEndian.Uint64(bz)
	}
	return id
}

func (k Keeper) autoIncrementID(ctx sdk.Context, lastIDKey []byte) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(lastIDKey)
	id := uint64(1)
	if bz != nil {
		id = binary.BigEndian.Uint64(bz)
	}
	bz = sdk.Uint64ToBigEndian(id + 1)
	store.Set(lastIDKey, bz)
	return id
}

func addrFromUint64(id uint64) sdk.AccAddress {
	addr := make([]byte, 20)
	addr[0] = 'C'
	binary.PutUvarint(addr[1:], id)
	return sdk.AccAddress(crypto.AddressHash(addr))
}
