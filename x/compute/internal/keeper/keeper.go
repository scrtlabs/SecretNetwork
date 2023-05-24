package keeper

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	transfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
	channelkeeper "github.com/cosmos/ibc-go/v4/modules/core/04-channel/keeper"
	portkeeper "github.com/cosmos/ibc-go/v4/modules/core/05-port/keeper"
	wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	"golang.org/x/crypto/ripemd160"

	"github.com/cosmos/cosmos-sdk/telemetry"

	"github.com/cosmos/cosmos-sdk/baseapp"
	codedctypes "github.com/cosmos/cosmos-sdk/codec/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	sdktxsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	wasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm"

	v010wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"
	v1wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v1"

	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

type emergencyButton interface {
	IsHalted(ctx sdk.Context) bool
}

type ResponseHandler interface {
	// Handle processes the data returned by a contract invocation.
	Handle(
		ctx sdk.Context,
		contractAddr sdk.AccAddress,
		ibcPort string,
		messages []v1wasmTypes.SubMsg,
		origRspData []byte,
		ogTx []byte,
		sigInfo wasmTypes.VerificationInfo,
	) ([]byte, error)
}

// Keeper will have a reference to Wasmer with it's own data directory.
type Keeper struct {
	storeKey         sdk.StoreKey
	cdc              codec.BinaryCodec
	legacyAmino      codec.LegacyAmino
	accountKeeper    authkeeper.AccountKeeper
	bankKeeper       bankkeeper.Keeper
	portKeeper       portkeeper.Keeper
	capabilityKeeper capabilitykeeper.ScopedKeeper
	wasmer           wasm.Wasmer
	queryPlugins     QueryPlugins
	messenger        Messenger
	// queryGasLimit is the max wasm gas that can be spent on executing a query with a contract
	queryGasLimit uint64
	HomeDir       string
	// authZPolicy   AuthorizationPolicy
	// paramSpace    subspace.Subspace
	LastMsgManager *baseapp.LastMsgMarkerContainer
}

func moduleLogger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// MessageRouter ADR 031 request type routing
type MessageRouter interface {
	Handler(msg sdk.Msg) baseapp.MsgServiceHandler
}

func NewKeeper(
	cdc codec.Codec,
	legacyAmino codec.LegacyAmino,
	storeKey sdk.StoreKey,
	accountKeeper authkeeper.AccountKeeper,
	bankKeeper bankkeeper.Keeper,
	govKeeper govkeeper.Keeper,
	distKeeper distrkeeper.Keeper,
	mintKeeper mintkeeper.Keeper,
	stakingKeeper stakingkeeper.Keeper,
	capabilityKeeper capabilitykeeper.ScopedKeeper,
	portKeeper portkeeper.Keeper,
	portSource types.ICS20TransferPortSource,
	channelKeeper channelkeeper.Keeper,
	ics4Wrapper transfertypes.ICS4Wrapper,
	legacyMsgRouter sdk.Router,
	msgRouter MessageRouter,
	queryRouter GRPCQueryRouter,
	homeDir string,
	wasmConfig *types.WasmConfig,
	supportedFeatures string,
	customEncoders *MessageEncoders,
	customPlugins *QueryPlugins,
	LastMsgManager *baseapp.LastMsgMarkerContainer,
) Keeper {
	wasmer, err := wasm.NewWasmer(filepath.Join(homeDir, "wasm"), supportedFeatures, wasmConfig.CacheSize, wasmConfig.EnclaveCacheSize)
	if err != nil {
		panic(err)
	}

	keeper := Keeper{
		storeKey:         storeKey,
		cdc:              cdc,
		legacyAmino:      legacyAmino,
		wasmer:           *wasmer,
		accountKeeper:    accountKeeper,
		bankKeeper:       bankKeeper,
		portKeeper:       portKeeper,
		capabilityKeeper: capabilityKeeper,
		messenger: NewMessageHandler(
			msgRouter,
			legacyMsgRouter,
			customEncoders,
			channelKeeper,
			ics4Wrapper,
			capabilityKeeper,
			portSource,
			cdc,
		),
		queryGasLimit:  wasmConfig.SmartQueryGasLimit,
		HomeDir:        homeDir,
		LastMsgManager: LastMsgManager,
	}
	keeper.queryPlugins = DefaultQueryPlugins(govKeeper, distKeeper, mintKeeper, bankKeeper, stakingKeeper, queryRouter, &keeper, channelKeeper).Merge(customPlugins)

	return keeper
}

func (k Keeper) GetLastMsgMarkerContainer() *baseapp.LastMsgMarkerContainer {
	return k.LastMsgManager
}

// Create uploads and compiles a WASM contract, returning a short identifier for the contract
func (k Keeper) Create(ctx sdk.Context, creator sdk.AccAddress, wasmCode []byte, source string, builder string) (codeID uint64, err error) {
	wasmCode, err = uncompress(wasmCode)
	if err != nil {
		return 0, sdkerrors.Wrap(types.ErrCreateFailed, err.Error())
	}
	ctx.GasMeter().ConsumeGas(types.CompileCost*uint64(len(wasmCode)), "Compiling WASM Bytecode")

	codeHash, err := k.wasmer.Create(wasmCode)
	if err != nil {
		return 0, sdkerrors.Wrap(types.ErrCreateFailed, err.Error())
	}
	store := ctx.KVStore(k.storeKey)
	codeID = k.autoIncrementID(ctx, types.KeyLastCodeID)

	codeInfo := types.NewCodeInfo(codeHash, creator, source, builder)
	// 0x01 | codeID (uint64) -> ContractInfo
	store.Set(types.GetCodeKey(codeID), k.cdc.MustMarshal(&codeInfo))

	return codeID, nil
}

func (k Keeper) importCode(ctx sdk.Context, codeID uint64, codeInfo types.CodeInfo, wasmCode []byte) error {
	wasmCode, err := uncompress(wasmCode)
	if err != nil {
		return sdkerrors.Wrap(types.ErrCreateFailed, err.Error())
	}
	newCodeHash, err := k.wasmer.Create(wasmCode)
	if err != nil {
		return sdkerrors.Wrap(types.ErrCreateFailed, err.Error())
	}
	if !bytes.Equal(codeInfo.CodeHash, newCodeHash) {
		return sdkerrors.Wrap(types.ErrInvalid, "code hashes not same")
	}

	store := ctx.KVStore(k.storeKey)
	key := types.GetCodeKey(codeID)
	if store.Has(key) {
		return sdkerrors.Wrapf(types.ErrDuplicate, "duplicate code: %d", codeID)
	}
	// 0x01 | codeID (uint64) -> ContractInfo
	store.Set(key, k.cdc.MustMarshal(&codeInfo))
	return nil
}

func (k Keeper) GetSignerInfo(ctx sdk.Context, signer sdk.AccAddress) ([]byte, sdktxsigning.SignMode, []byte, []byte, []byte, error) {
	tx := sdktx.Tx{}
	err := k.cdc.Unmarshal(ctx.TxBytes(), &tx)
	if err != nil {
		return nil, 0, nil, nil, nil, sdkerrors.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to decode transaction from bytes: %s", err.Error()))
	}

	// for MsgInstantiateContract, there is only one signer which is msg.Sender
	// (https://github.com/scrtlabs/SecretNetwork/blob/d7813792fa07b93a10f0885eaa4c5e0a0a698854/x/compute/internal/types/msg.go#L192-L194)
	signerAcc, err := ante.GetSignerAcc(ctx, k.accountKeeper, signer)
	if err != nil {
		return nil, 0, nil, nil, nil, sdkerrors.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to retrieve account by address: %s", err.Error()))
	}

	txConfig := authtx.NewTxConfig(k.cdc.(*codec.ProtoCodec), authtx.DefaultSignModes)
	modeHandler := txConfig.SignModeHandler()
	signingData := authsigning.SignerData{
		ChainID:       ctx.ChainID(),
		AccountNumber: signerAcc.GetAccountNumber(),
		Sequence:      signerAcc.GetSequence() - 1,
	}

	protobufTx := authtx.WrapTx(&tx).GetTx()

	pubKeys, err := protobufTx.GetPubKeys()
	if err != nil {
		return nil, 0, nil, nil, nil, sdkerrors.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to get public keys for instantiate: %s", err.Error()))
	}

	pkIndex := -1
	var _signers [][]byte // This is just used for the error message below
	for index, pubKey := range pubKeys {
		thisSigner := pubKey.Address().Bytes()
		_signers = append(_signers, thisSigner)
		if bytes.Equal(thisSigner, signer.Bytes()) {
			pkIndex = index
		}
	}
	if pkIndex == -1 {
		return nil, 0, nil, nil, nil, sdkerrors.Wrap(types.ErrSigFailed, fmt.Sprintf("Message sender: %v is not found in the tx signer set: %v, callback signature not provided", signer, _signers))
	}

	signatures, err := protobufTx.GetSignaturesV2()
	if err != nil {
		return nil, 0, nil, nil, nil, sdkerrors.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to get signatures: %s", err.Error()))
	}
	var signMode sdktxsigning.SignMode
	switch signData := signatures[pkIndex].Data.(type) {
	case *sdktxsigning.SingleSignatureData:
		signMode = signData.SignMode
	case *sdktxsigning.MultiSignatureData:
		signMode = sdktxsigning.SignMode_SIGN_MODE_LEGACY_AMINO_JSON
	}
	signBytes, err := modeHandler.GetSignBytes(signMode, signingData, protobufTx)
	if err != nil {
		return nil, 0, nil, nil, nil, sdkerrors.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to recreate sign bytes for the tx: %s", err.Error()))
	}

	modeInfoBytes, err := sdktxsigning.SignatureDataToProto(signatures[pkIndex].Data).Marshal()
	if err != nil {
		return nil, 0, nil, nil, nil, sdkerrors.Wrap(types.ErrSigFailed, "couldn't marshal mode info")
	}

	var pkBytes []byte
	pubKey := pubKeys[pkIndex]
	anyPubKey, err := codedctypes.NewAnyWithValue(pubKey)
	if err != nil {
		return nil, 0, nil, nil, nil, sdkerrors.Wrap(types.ErrSigFailed, "couldn't turn public key into Any")
	}
	pkBytes, err = k.cdc.Marshal(anyPubKey)
	if err != nil {
		return nil, 0, nil, nil, nil, sdkerrors.Wrap(types.ErrSigFailed, "couldn't marshal public key")
	}
	return signBytes, signMode, modeInfoBytes, pkBytes, tx.Signatures[pkIndex], nil
}

func V010MsgToV1SubMsg(contractAddress string, msg v010wasmTypes.CosmosMsg) (v1wasmTypes.SubMsg, error) {
	if !isValidV010Msg(msg) {
		return v1wasmTypes.SubMsg{}, fmt.Errorf("exactly one message type is supported: %+v", msg)
	}

	subMsg := v1wasmTypes.SubMsg{
		ID:       0,   // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0/packages/std/src/results/submessages.rs#L40-L41
		GasLimit: nil, // New v1 submessages module handles nil as unlimited, in v010 the gas was not limited for messages
		ReplyOn:  v1wasmTypes.ReplyNever,
	}

	if msg.Bank != nil { //nolint:gocritic
		if msg.Bank.Send.FromAddress != contractAddress {
			return v1wasmTypes.SubMsg{}, fmt.Errorf("contract doesn't have permission to send funds from another account (using BankMsg)")
		}
		subMsg.Msg = v1wasmTypes.CosmosMsg{
			Bank: &v1wasmTypes.BankMsg{
				Send: &v1wasmTypes.SendMsg{ToAddress: msg.Bank.Send.ToAddress, Amount: msg.Bank.Send.Amount},
			},
		}
	} else if msg.Custom != nil {
		subMsg.Msg.Custom = msg.Custom
	} else if msg.Staking != nil {
		subMsg.Msg = v1wasmTypes.CosmosMsg{
			Staking: &v1wasmTypes.StakingMsg{
				Delegate:   msg.Staking.Delegate,
				Undelegate: msg.Staking.Undelegate,
				Redelegate: msg.Staking.Redelegate,
				Withdraw:   msg.Staking.Withdraw,
			},
		}
	} else if msg.Wasm != nil {
		subMsg.Msg = v1wasmTypes.CosmosMsg{
			Wasm: &v1wasmTypes.WasmMsg{
				Execute:     msg.Wasm.Execute,
				Instantiate: msg.Wasm.Instantiate,
			},
		}
	} else if msg.Gov != nil {
		subMsg.Msg = v1wasmTypes.CosmosMsg{
			Gov: &v1wasmTypes.GovMsg{
				Vote: &v1wasmTypes.VoteMsg{ProposalId: msg.Gov.Vote.Proposal, Vote: v1wasmTypes.ToVoteOption[msg.Gov.Vote.VoteOption]},
			},
		}
	}

	return subMsg, nil
}

func V010MsgsToV1SubMsgs(contractAddr string, msgs []v010wasmTypes.CosmosMsg) ([]v1wasmTypes.SubMsg, error) {
	subMsgs := []v1wasmTypes.SubMsg{}
	for _, msg := range msgs {
		v1SubMsg, err := V010MsgToV1SubMsg(contractAddr, msg)
		if err != nil {
			return nil, err
		}
		subMsgs = append(subMsgs, v1SubMsg)
	}

	return subMsgs, nil
}

// Instantiate creates an instance of a WASM contract
func (k Keeper) Instantiate(ctx sdk.Context, codeID uint64, creator sdk.AccAddress, admin sdk.AccAddress, initMsg []byte, label string, deposit sdk.Coins, callbackSig []byte) (sdk.AccAddress, []byte, error) {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "instantiate")

	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading CosmWasm module: init")

	signBytes := []byte{}
	signMode := sdktxsigning.SignMode_SIGN_MODE_UNSPECIFIED
	modeInfoBytes := []byte{}
	pkBytes := []byte{}
	signerSig := []byte{}
	var err error

	// If no callback signature - we should send the actual msg sender sign bytes and signature
	if callbackSig == nil {
		signBytes, signMode, modeInfoBytes, pkBytes, signerSig, err = k.GetSignerInfo(ctx, creator)
		if err != nil {
			return nil, nil, err
		}
	}

	verificationInfo := types.NewVerificationInfo(signBytes, signMode, modeInfoBytes, pkBytes, signerSig, callbackSig)

	// create contract address

	store := ctx.KVStore(k.storeKey)
	existingAddress := store.Get(types.GetContractLabelPrefix(label))

	if existingAddress != nil {
		return nil, nil, sdkerrors.Wrap(types.ErrAccountExists, label)
	}

	contractAddress := k.generateContractAddress(ctx, codeID, creator)
	existingAcct := k.accountKeeper.GetAccount(ctx, contractAddress)
	if existingAcct != nil {
		return nil, nil, sdkerrors.Wrap(types.ErrAccountExists, existingAcct.GetAddress().String())
	}

	if admin == nil {
		admin = creator
	}

	// deposit initial contract funds
	if !deposit.IsZero() {
		if k.bankKeeper.BlockedAddr(creator) {
			return nil, nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, "blocked address can not be used")
		}
		sdkerr := k.bankKeeper.SendCoins(ctx, creator, contractAddress, deposit)
		if sdkerr != nil {
			return nil, nil, sdkerr
		}
	} else {
		// create an empty account (so we don't have issues later)
		// TODO: can we remove this?
		contractAccount := k.accountKeeper.NewAccountWithAddress(ctx, contractAddress)
		k.accountKeeper.SetAccount(ctx, contractAccount)
	}

	// get contact info
	bz := store.Get(types.GetCodeKey(codeID))
	if bz == nil {
		return nil, nil, sdkerrors.Wrap(types.ErrNotFound, "code")
	}
	var codeInfo types.CodeInfo
	k.cdc.MustUnmarshal(bz, &codeInfo)

	random := k.GetRandomSeed(ctx, ctx.BlockHeight())

	// prepare env for contract instantiate call
	env := types.NewEnv(ctx, creator, deposit, contractAddress, nil, random)

	// create prefixed data store
	// 0x03 | contractAddress (sdk.AccAddress)
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), prefixStoreKey)

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
		Caller:  contractAddress,
	}

	// instantiate wasm contract
	gas := gasForContract(ctx)
	response, key, adminProof, gasUsed, err := k.wasmer.Instantiate(codeInfo.CodeHash, env, initMsg, prefixStore, cosmwasmAPI, querier, ctx.GasMeter(), gas, verificationInfo, admin)
	consumeGas(ctx, gasUsed)

	if err != nil {
		switch res := response.(type) { //nolint:gocritic
		case v1wasmTypes.DataWithInternalReplyInfo:
			result, e := json.Marshal(res)
			if e != nil {
				return nil, nil, sdkerrors.Wrap(e, "couldn't marshal internal reply info")
			}

			return contractAddress, result, sdkerrors.Wrap(types.ErrInstantiateFailed, err.Error())
		}

		return contractAddress, nil, sdkerrors.Wrap(types.ErrInstantiateFailed, err.Error())
	}

	switch res := response.(type) {
	case *v010wasmTypes.InitResponse:
		// emit all events from this contract itself

		// persist instance
		createdAt := types.NewAbsoluteTxPosition(ctx)
		contractInfo := types.NewContractInfo(codeID, creator, admin, adminProof, label, createdAt)

		historyEntry := contractInfo.InitialHistory(initMsg)
		k.addToContractCodeSecondaryIndex(ctx, contractAddress, historyEntry)
		//k.addToContractCreatorSecondaryIndex(ctx, creator, historyEntry.Updated, contractAddress)
		k.appendToContractHistory(ctx, contractAddress, historyEntry)

		k.setContractInfo(ctx, contractAddress, &contractInfo)
		k.SetContractKey(ctx, contractAddress, &types.ContractKey{
			Key: key,
		})
		store.Set(types.GetContractLabelPrefix(label), contractAddress)

		subMessages, err := V010MsgsToV1SubMsgs(contractAddress.String(), res.Messages)
		if err != nil {
			return nil, nil, sdkerrors.Wrap(err, "couldn't convert v0.10 messages to v1 messages")
		}

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, subMessages, res.Log, []v1wasmTypes.Event{}, res.Data, initMsg, verificationInfo, wasmTypes.CosmosMsgVersionV010)
		if err != nil {
			return nil, nil, sdkerrors.Wrap(err, "dispatch")
		}

		return contractAddress, data, nil
	case *v1wasmTypes.Response:
		// persist instance first
		createdAt := types.NewAbsoluteTxPosition(ctx)
		contractInfo := types.NewContractInfo(codeID, creator, admin, adminProof, label, createdAt)

		// check for IBC flag
		report, err := k.wasmer.AnalyzeCode(codeInfo.CodeHash)
		if err != nil {
			return contractAddress, nil, sdkerrors.Wrap(types.ErrInstantiateFailed, err.Error())
		}
		if report.HasIBCEntryPoints {
			// register IBC port
			ibcPort, err := k.ensureIbcPort(ctx, contractAddress)
			if err != nil {
				return nil, nil, err
			}
			contractInfo.IBCPortID = ibcPort
		}

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeInstantiate,
			sdk.NewAttribute(types.AttributeKeyContractAddr, contractAddress.String()),
			sdk.NewAttribute(types.AttributeKeyCodeID, strconv.FormatUint(codeID, 10)),
		))

		historyEntry := contractInfo.InitialHistory(initMsg)
		k.addToContractCodeSecondaryIndex(ctx, contractAddress, historyEntry)
		//k.addToContractCreatorSecondaryIndex(ctx, creator, historyEntry.Updated, contractAddress)
		k.appendToContractHistory(ctx, contractAddress, historyEntry)

		// persist instance
		k.setContractInfo(ctx, contractAddress, &contractInfo)
		k.SetContractKey(ctx, contractAddress, &types.ContractKey{
			Key: key,
		})
		store.Set(types.GetContractLabelPrefix(label), contractAddress)

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, res.Messages, res.Attributes, res.Events, res.Data, initMsg, verificationInfo, wasmTypes.CosmosMsgVersionV1)
		if err != nil {
			return nil, nil, sdkerrors.Wrap(err, "dispatch")
		}

		return contractAddress, data, nil
	default:
		return nil, nil, sdkerrors.Wrap(types.ErrInstantiateFailed, fmt.Sprintf("cannot detect response type: %+v", res))
	}
}

// Execute executes the contract instance
func (k Keeper) Execute(ctx sdk.Context, contractAddress sdk.AccAddress, caller sdk.AccAddress, msg []byte, coins sdk.Coins, callbackSig []byte) (*sdk.Result, error) {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "execute")

	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading Compute module: execute")

	signBytes := []byte{}
	signMode := sdktxsigning.SignMode_SIGN_MODE_UNSPECIFIED
	modeInfoBytes := []byte{}
	pkBytes := []byte{}
	signerSig := []byte{}
	var err error

	// If no callback signature - we should send the actual msg sender sign bytes and signature
	if callbackSig == nil {
		signBytes, signMode, modeInfoBytes, pkBytes, signerSig, err = k.GetSignerInfo(ctx, caller)
		if err != nil {
			return nil, err
		}
	}

	verificationInfo := types.NewVerificationInfo(signBytes, signMode, modeInfoBytes, pkBytes, signerSig, callbackSig)

	contractInfo, codeInfo, prefixStore, err := k.contractInstance(ctx, contractAddress)
	if err != nil {
		return nil, err
	}

	// add more funds
	if !coins.IsZero() {
		if k.bankKeeper.BlockedAddr(caller) {
			return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, "blocked address can not be used")
		}

		sdkerr := k.bankKeeper.SendCoins(ctx, caller, contractAddress, coins)
		if sdkerr != nil {
			return nil, sdkerr
		}
	}

	random := k.GetRandomSeed(ctx, ctx.BlockHeight())

	contractKey, err := k.GetContractKey(ctx, contractAddress)
	if err != nil {
		return nil, err
	}

	env := types.NewEnv(ctx, caller, coins, contractAddress, &contractKey, random)

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
		Caller:  contractAddress,
	}

	gas := gasForContract(ctx)
	response, gasUsed, execErr := k.wasmer.Execute(codeInfo.CodeHash, env, msg, prefixStore, cosmwasmAPI, querier, gasMeter(ctx), gas, verificationInfo, wasmTypes.HandleTypeExecute)
	consumeGas(ctx, gasUsed)

	if execErr != nil {
		var result sdk.Result
		switch res := response.(type) { //nolint:gocritic
		case v1wasmTypes.DataWithInternalReplyInfo:
			result.Data, err = json.Marshal(res)
			if err != nil {
				return nil, sdkerrors.Wrap(err, "couldn't marshal internal reply info")
			}
		}

		return &result, sdkerrors.Wrap(types.ErrExecuteFailed, execErr.Error())
	}

	switch res := response.(type) {
	case *v010wasmTypes.HandleResponse:
		subMessages, err := V010MsgsToV1SubMsgs(contractAddress.String(), res.Messages)
		if err != nil {
			return nil, sdkerrors.Wrap(err, "couldn't convert v0.10 messages to v1 messages")
		}

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, subMessages, res.Log, []v1wasmTypes.Event{}, res.Data, msg, verificationInfo, wasmTypes.CosmosMsgVersionV010)
		if err != nil {
			return nil, sdkerrors.Wrap(err, "dispatch")
		}

		return &sdk.Result{
			Data: data,
		}, nil
	case *v1wasmTypes.Response:
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeExecute,
			sdk.NewAttribute(types.AttributeKeyContractAddr, contractAddress.String()),
		))

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, res.Messages, res.Attributes, res.Events, res.Data, msg, verificationInfo, wasmTypes.CosmosMsgVersionV1)
		if err != nil {
			return nil, sdkerrors.Wrap(err, "dispatch")
		}

		return &sdk.Result{
			Data: data,
		}, nil
	default:
		return nil, sdkerrors.Wrap(types.ErrExecuteFailed, fmt.Sprintf("cannot detect response type: %+v", res))
	}
}

// QuerySmart queries the smart contract itself.
func (k Keeper) QuerySmart(ctx sdk.Context, contractAddr sdk.AccAddress, req []byte, useDefaultGasLimit bool) ([]byte, error) {
	return k.querySmartImpl(ctx, contractAddr, req, useDefaultGasLimit, 1)
}

// QuerySmartRecursive queries the smart contract itself. This should only be called when running inside another query recursively.
func (k Keeper) querySmartRecursive(ctx sdk.Context, contractAddr sdk.AccAddress, req []byte, queryDepth uint32, useDefaultGasLimit bool) ([]byte, error) {
	return k.querySmartImpl(ctx, contractAddr, req, useDefaultGasLimit, queryDepth)
}

func (k Keeper) querySmartImpl(ctx sdk.Context, contractAddress sdk.AccAddress, req []byte, useDefaultGasLimit bool, queryDepth uint32) ([]byte, error) {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "query")

	if useDefaultGasLimit {
		ctx = ctx.WithGasMeter(sdk.NewGasMeter(k.queryGasLimit))
	}

	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading CosmWasm module: query")

	_, codeInfo, prefixStore, err := k.contractInstance(ctx, contractAddress)
	if err != nil {
		return nil, err
	}

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
		Caller:  contractAddress,
	}

	contractKey, err := k.GetContractKey(ctx, contractAddress)
	if err != nil {
		return nil, err
	}

	params := types.NewEnv(
		ctx,
		sdk.AccAddress{}, /* empty because it's unused in queries */
		sdk.NewCoins(),   /* empty because it's unused in queries */
		contractAddress,
		&contractKey,
		[]byte{0}, /* empty because it's unused in queries */
	)
	params.QueryDepth = queryDepth

	queryResult, gasUsed, qErr := k.wasmer.Query(codeInfo.CodeHash, params, req, prefixStore, cosmwasmAPI, querier, gasMeter(ctx), gasForContract(ctx))
	consumeGas(ctx, gasUsed)

	telemetry.SetGauge(float32(gasUsed), "compute", "keeper", "query", contractAddress.String(), "gasUsed")

	if qErr != nil {
		return nil, sdkerrors.Wrap(types.ErrQueryFailed, qErr.Error())
	}
	return queryResult, nil
}

// We don't use this function since we have an encrypted state. It's here for upstream compatibility
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

func (k Keeper) contractInstance(ctx sdk.Context, contractAddress sdk.AccAddress) (types.ContractInfo, types.CodeInfo, prefix.Store, error) {
	store := ctx.KVStore(k.storeKey)

	contractBz := store.Get(types.GetContractAddressKey(contractAddress))
	if contractBz == nil {
		return types.ContractInfo{}, types.CodeInfo{}, prefix.Store{}, sdkerrors.Wrap(types.ErrNotFound, "contract")
	}
	var contract types.ContractInfo
	k.cdc.MustUnmarshal(contractBz, &contract)

	contractInfoBz := store.Get(types.GetCodeKey(contract.CodeID))
	if contractInfoBz == nil {
		return types.ContractInfo{}, types.CodeInfo{}, prefix.Store{}, sdkerrors.Wrap(types.ErrNotFound, "contract info")
	}
	var codeInfo types.CodeInfo
	k.cdc.MustUnmarshal(contractInfoBz, &codeInfo)
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), prefixStoreKey)
	return contract, codeInfo, prefixStore, nil
}

func (k Keeper) GetContractKey(ctx sdk.Context, contractAddress sdk.AccAddress) (types.ContractKey, error) {
	store := ctx.KVStore(k.storeKey)

	var contractKey types.ContractKey
	contractKeyBz := store.Get(types.GetContractEnclaveKey(contractAddress))

	if contractKeyBz == nil {
		return types.ContractKey{}, sdkerrors.Wrap(types.ErrNotFound, "contract key")
	}

	err := k.cdc.Unmarshal(contractKeyBz, &contractKey)
	if err != nil {
		return contractKey, err
	}

	return contractKey, nil
}

func (k Keeper) SetContractKey(ctx sdk.Context, contractAddress sdk.AccAddress, contractKey *types.ContractKey) {
	store := ctx.KVStore(k.storeKey)

	contractKeyBz := k.cdc.MustMarshal(contractKey)
	store.Set(types.GetContractEnclaveKey(contractAddress), contractKeyBz)
}

func (k Keeper) GetRandomSeed(ctx sdk.Context, height int64) []byte {
	store := ctx.KVStore(k.storeKey)

	random := store.Get(types.GetRandomKey(height))

	return random
}

func (k Keeper) SetRandomSeed(ctx sdk.Context, random []byte) {
	store := ctx.KVStore(k.storeKey)

	ctx.Logger().Info(fmt.Sprintf("Setting random: %s", hex.EncodeToString(random)))

	store.Set(types.GetRandomKey(ctx.BlockHeight()), random)
}

func (k Keeper) GetContractAddress(ctx sdk.Context, label string) sdk.AccAddress {
	store := ctx.KVStore(k.storeKey)

	contractAddress := store.Get(types.GetContractLabelPrefix(label))

	return contractAddress
}

func (k Keeper) GetContractHash(ctx sdk.Context, contractAddress sdk.AccAddress) ([]byte, error) {
	contractInfo := k.GetContractInfo(ctx, contractAddress)

	if contractInfo == nil {
		return nil, fmt.Errorf("failed to get contract info for the following address: %s", contractAddress.String())
	}

	codeId := contractInfo.CodeID

	codeInfo, err := k.GetCodeInfo(ctx, codeId)
	if err != nil {
		return nil, err
	}

	return codeInfo.CodeHash, nil
}

func (k Keeper) GetContractInfo(ctx sdk.Context, contractAddress sdk.AccAddress) *types.ContractInfo {
	store := ctx.KVStore(k.storeKey)
	var contract types.ContractInfo
	contractBz := store.Get(types.GetContractAddressKey(contractAddress))
	if contractBz == nil {
		return nil
	}
	k.cdc.MustUnmarshal(contractBz, &contract)
	return &contract
}

func (k Keeper) containsContractInfo(ctx sdk.Context, contractAddress sdk.AccAddress) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.GetContractAddressKey(contractAddress))
}

func (k Keeper) setContractInfo(ctx sdk.Context, contractAddress sdk.AccAddress, contract *types.ContractInfo) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetContractAddressKey(contractAddress), k.cdc.MustMarshal(contract))
}

func (k Keeper) setContractCustomInfo(ctx sdk.Context, contractAddress sdk.AccAddress, contract *types.ContractCustomInfo) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetContractEnclaveKey(contractAddress), contract.EnclaveKey)
	// println(fmt.Sprintf("Setting enclave key: %x: %x\n", types.GetContractEnclaveKey(contractAddress), contract.EnclaveKey))
	store.Set(types.GetContractLabelPrefix(contract.Label), contractAddress)
	// println(fmt.Sprintf("Setting label: %x: %x\n", types.GetContractLabelPrefix(contract.Label), contractAddress))
}

func (k Keeper) IterateContractInfo(ctx sdk.Context, cb func(sdk.AccAddress, types.ContractInfo, types.ContractCustomInfo) bool) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.ContractKeyPrefix)
	iter := prefixStore.Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		var contract types.ContractInfo
		k.cdc.MustUnmarshal(iter.Value(), &contract)

		enclaveId := ctx.KVStore(k.storeKey).Get(types.GetContractEnclaveKey(iter.Key()))
		// println(fmt.Sprintf("Setting enclave key: %x: %x\n", types.GetContractEnclaveKey(iter.Key()), enclaveId))
		// println(fmt.Sprintf("Setting label: %x: %x\n", types.GetContractLabelPrefix(contract.Label), contract.Label))

		contractCustomInfo := types.ContractCustomInfo{
			EnclaveKey: enclaveId,
			Label:      contract.Label,
		}

		// cb returns true to stop early
		if cb(iter.Key(), contract, contractCustomInfo) {
			break
		}
	}
}

func (k Keeper) GetContractState(ctx sdk.Context, contractAddress sdk.AccAddress) sdk.Iterator {
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), prefixStoreKey)
	return prefixStore.Iterator(nil, nil)
}

func (k Keeper) importContractState(ctx sdk.Context, contractAddress sdk.AccAddress, models []types.Model) error {
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), prefixStoreKey)
	for _, model := range models {
		if model.Value == nil {
			model.Value = []byte{}
		}

		if prefixStore.Has(model.Key) {
			return sdkerrors.Wrapf(types.ErrDuplicate, "duplicate key: %x", model.Key)
		}
		prefixStore.Set(model.Key, model.Value)

	}
	return nil
}

func (k Keeper) GetCodeInfo(ctx sdk.Context, codeID uint64) (types.CodeInfo, error) {
	store := ctx.KVStore(k.storeKey)
	var codeInfo types.CodeInfo
	codeInfoBz := store.Get(types.GetCodeKey(codeID))
	if codeInfoBz == nil {
		return types.CodeInfo{}, fmt.Errorf("failed to get code info for code id %d", codeID)
	}
	k.cdc.MustUnmarshal(codeInfoBz, &codeInfo)
	return codeInfo, nil
}

func (k Keeper) containsCodeInfo(ctx sdk.Context, codeID uint64) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.GetCodeKey(codeID))
}

func (k Keeper) IterateCodeInfos(ctx sdk.Context, cb func(uint64, types.CodeInfo) bool) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.CodeKeyPrefix)
	iter := prefixStore.Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		var c types.CodeInfo
		k.cdc.MustUnmarshal(iter.Value(), &c)
		// cb returns true to stop early
		if cb(binary.BigEndian.Uint64(iter.Key()), c) {
			return
		}
	}
}

func (k Keeper) GetWasm(ctx sdk.Context, codeID uint64) ([]byte, error) {
	store := ctx.KVStore(k.storeKey)
	var codeInfo types.CodeInfo
	codeInfoBz := store.Get(types.GetCodeKey(codeID))
	if codeInfoBz == nil {
		return nil, nil
	}
	k.cdc.MustUnmarshal(codeInfoBz, &codeInfo)
	return k.wasmer.GetCode(codeInfo.CodeHash)
}

// handleContractResponse processes the contract response data by emitting events and sending sub-/messages.
func (k *Keeper) handleContractResponse(
	ctx sdk.Context,
	contractAddr sdk.AccAddress,
	ibcPort string,
	msgs []v1wasmTypes.SubMsg,
	logs []v010wasmTypes.LogAttribute,
	evts v1wasmTypes.Events,
	data []byte,
// original TX in order to extract the first 64bytes of signing info
	ogTx []byte,
// sigInfo of the initial message that triggered the original contract call
// This is used mainly in replies in order to decrypt their data.
	ogSigInfo wasmTypes.VerificationInfo,
	ogCosmosMessageVersion wasmTypes.CosmosMsgVersion,
) ([]byte, error) {
	events := types.ContractLogsToSdkEvents(logs, contractAddr)

	ctx.EventManager().EmitEvents(events)

	if len(evts) > 0 {

		customEvents, err := types.NewCustomEvents(evts, contractAddr)
		if err != nil {
			return nil, err
		}

		ctx.EventManager().EmitEvents(customEvents)
	}

	responseHandler := NewContractResponseHandler(NewMessageDispatcher(k.messenger, k))
	return responseHandler.Handle(ctx, contractAddr, ibcPort, msgs, data, ogTx, ogSigInfo, ogCosmosMessageVersion)
}

func gasForContract(ctx sdk.Context) uint64 {
	meter := ctx.GasMeter()
	remaining := (meter.Limit() - meter.GasConsumed()) * types.GasMultiplier
	if remaining > types.MaxGas {
		return types.MaxGas
	}
	return remaining
}

func consumeGas(ctx sdk.Context, gas uint64) {
	consumed := (gas / types.GasMultiplier) + 1
	ctx.GasMeter().ConsumeGas(consumed, "wasm contract")
	// throw OutOfGas error if we ran out (got exactly to zero due to better limit enforcing)
	if ctx.GasMeter().IsOutOfGas() {
		panic(sdk.ErrorOutOfGas{Descriptor: "Wasmer function execution"})
	}
}

// generates a contract address from codeID + instanceID
func (k Keeper) generateContractAddress(ctx sdk.Context, codeID uint64, creator sdk.AccAddress) sdk.AccAddress {
	instanceID := k.autoIncrementID(ctx, types.KeyLastInstanceID)
	return contractAddress(codeID, instanceID, creator)
}

func contractAddress(codeID, instanceID uint64, creator sdk.AccAddress) sdk.AccAddress {
	contractId := codeID<<32 + instanceID
	contractIdBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(contractIdBytes, contractId)

	sourceBytes := append(contractIdBytes, creator...)

	sha := sha256.Sum256(sourceBytes)
	hasherRIPEMD160 := ripemd160.New()
	hasherRIPEMD160.Write(sha[:]) // does not error
	return sdk.AccAddress(hasherRIPEMD160.Sum(nil))
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

// peekAutoIncrementID reads the current value without incrementing it.
func (k Keeper) peekAutoIncrementID(ctx sdk.Context, lastIDKey []byte) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(lastIDKey)
	id := uint64(1)
	if bz != nil {
		id = binary.BigEndian.Uint64(bz)
	}
	return id
}

func (k Keeper) importAutoIncrementID(ctx sdk.Context, lastIDKey []byte, val uint64) error {
	store := ctx.KVStore(k.storeKey)
	if store.Has(lastIDKey) {
		return sdkerrors.Wrapf(types.ErrDuplicate, "autoincrement id: %s", string(lastIDKey))
	}
	bz := sdk.Uint64ToBigEndian(val)
	store.Set(lastIDKey, bz)
	return nil
}

func (k Keeper) importContract(ctx sdk.Context, contractAddr sdk.AccAddress, customInfo *types.ContractCustomInfo, c *types.ContractInfo, state []types.Model) error {
	if !k.containsCodeInfo(ctx, c.CodeID) {
		return sdkerrors.Wrapf(types.ErrNotFound, "code id: %d", c.CodeID)
	}
	if k.containsContractInfo(ctx, contractAddr) {
		return sdkerrors.Wrapf(types.ErrDuplicate, "contract: %s", contractAddr)
	}

	k.setContractCustomInfo(ctx, contractAddr, customInfo)
	k.setContractInfo(ctx, contractAddr, c)
	return k.importContractState(ctx, contractAddr, state)
}

// MultipliedGasMeter wraps the GasMeter from context and multiplies all reads by out defined multiplier
type MultipiedGasMeter struct {
	originalMeter sdk.GasMeter
}

var _ wasm.GasMeter = MultipiedGasMeter{}

func (m MultipiedGasMeter) GasConsumed() sdk.Gas {
	return m.originalMeter.GasConsumed() * types.GasMultiplier
}

func gasMeter(ctx sdk.Context) MultipiedGasMeter {
	return MultipiedGasMeter{
		originalMeter: ctx.GasMeter(),
	}
}

type MsgDispatcher interface {
	DispatchSubmessages(ctx sdk.Context, contractAddr sdk.AccAddress, ibcPort string, msgs []v1wasmTypes.SubMsg, ogTx []byte, ogSigInfo wasmTypes.VerificationInfo, ogCosmosMessageVersion wasmTypes.CosmosMsgVersion) ([]byte, error)
}

// ContractResponseHandler default implementation that first dispatches submessage then normal messages.
// The Submessage execution may include an success/failure response handling by the contract that can overwrite the
// original
type ContractResponseHandler struct {
	md MsgDispatcher
}

func NewContractResponseHandler(md MsgDispatcher) *ContractResponseHandler {
	return &ContractResponseHandler{md: md}
}

// Handle processes the data returned by a contract invocation.
func (h ContractResponseHandler) Handle(ctx sdk.Context, contractAddr sdk.AccAddress, ibcPort string, messages []v1wasmTypes.SubMsg, origRspData []byte, ogTx []byte, ogSigInfo wasmTypes.VerificationInfo, ogCosmosMessageVersion wasmTypes.CosmosMsgVersion) ([]byte, error) {
	result := origRspData
	switch rsp, err := h.md.DispatchSubmessages(ctx, contractAddr, ibcPort, messages, ogTx, ogSigInfo, ogCosmosMessageVersion); {
	case err != nil:
		return nil, sdkerrors.Wrap(err, "submessages")
	case rsp != nil:
		result = rsp
	}
	return result, nil
}

// reply is only called from keeper internal functions (dispatchSubmessages) after processing the submessage
func (k Keeper) reply(ctx sdk.Context, contractAddress sdk.AccAddress, reply v1wasmTypes.Reply, ogTx []byte, ogSigInfo wasmTypes.VerificationInfo) ([]byte, error) {
	contractInfo, codeInfo, prefixStore, err := k.contractInstance(ctx, contractAddress)
	if err != nil {
		return nil, err
	}

	// always consider this pinned
	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading Compute module: reply")

	contractKey, err := k.GetContractKey(ctx, contractAddress)
	if err != nil {
		return nil, err
	}

	random := k.GetRandomSeed(ctx, ctx.BlockHeight())

	env := types.NewEnv(ctx, contractAddress, sdk.Coins{}, contractAddress, &contractKey, random)

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
		Caller:  contractAddress,
	}

	// instantiate wasm contract
	gas := gasForContract(ctx)
	marshaledReply, error := json.Marshal(reply)
	marshaledReply = append(ogTx[0:64], marshaledReply...)

	if error != nil {
		return nil, error
	}

	response, gasUsed, execErr := k.wasmer.Execute(codeInfo.CodeHash, env, marshaledReply, prefixStore, cosmwasmAPI, querier, ctx.GasMeter(), gas, ogSigInfo, wasmTypes.HandleTypeReply)
	consumeGas(ctx, gasUsed)

	if execErr != nil {
		return nil, sdkerrors.Wrap(types.ErrReplyFailed, execErr.Error())
	}

	switch res := response.(type) {
	case *v010wasmTypes.HandleResponse:
		return nil, sdkerrors.Wrap(types.ErrReplyFailed, fmt.Sprintf("response of reply should always be a CosmWasm v1 response type: %+v", res))
	case *v1wasmTypes.Response:
		consumeGas(ctx, gasUsed)

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeReply,
			sdk.NewAttribute(types.AttributeKeyContractAddr, contractAddress.String()),
		))

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, res.Messages, res.Attributes, res.Events, res.Data, ogTx, ogSigInfo, wasmTypes.CosmosMsgVersionV1)
		if err != nil {
			return nil, sdkerrors.Wrap(types.ErrReplyFailed, err.Error())
		}

		return data, nil
	default:
		return nil, sdkerrors.Wrap(types.ErrReplyFailed, fmt.Sprintf("cannot detect response type: %+v", res))
	}
}

func (k Keeper) UpdateContractAdmin(ctx sdk.Context, contractAddress, caller, newAdmin sdk.AccAddress) error {
	contractInfo := k.GetContractInfo(ctx, contractAddress)
	if contractInfo == nil {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "unknown contract")
	}
	contractInfo.Admin = newAdmin
	k.setContractInfo(ctx, contractAddress, contractInfo)
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeUpdateContractAdmin,
		sdk.NewAttribute(types.AttributeKeyContractAddr, contractAddress.String()),
		sdk.NewAttribute(types.AttributeKeyNewAdmin, newAdmin.String()),
	))

	return nil
}

func (k Keeper) Migrate(ctx sdk.Context, contractAddress sdk.AccAddress, caller sdk.AccAddress, newCodeID uint64, msg []byte) ([]byte, error) {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "migrate")
	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading CosmWasm module: migrate")

	signBytes, signMode, modeInfoBytes, pkBytes, signerSig, err := k.GetSignerInfo(ctx, caller)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, err.Error())
	}

	verificationInfo := types.NewVerificationInfo(signBytes, signMode, modeInfoBytes, pkBytes, signerSig, nil)

	contractInfo, _, prefixStore, err := k.contractInstance(ctx, contractAddress)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, sdkerrors.Wrap(err, "unknown contract").Error())
	}

	newCodeInfo, err := k.GetCodeInfo(ctx, newCodeID)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, sdkerrors.Wrap(err, "unknown code").Error())
	}

	// check for IBC flag
	switch report, err := k.wasmer.AnalyzeCode(newCodeInfo.CodeHash); {
	case err != nil:
		return nil, sdkerrors.Wrap(types.ErrMigrationFailed, err.Error())
	case !report.HasIBCEntryPoints && contractInfo.IBCPortID != "":
		// prevent update of ibc contract to non ibc contract
		return nil, sdkerrors.Wrap(types.ErrMigrationFailed, "requires ibc callbacks")
	case report.HasIBCEntryPoints && contractInfo.IBCPortID == "":
		// add ibc port
		ibcPort, err := k.ensureIbcPort(ctx, contractAddress)
		if err != nil {
			return nil, err
		}
		contractInfo.IBCPortID = ibcPort
	}

	contractKey, err := k.GetContractKey(ctx, contractAddress)
	if err != nil {
		return nil, err
	}

	if contractInfo.Admin.String() != caller.String() {
		return nil, sdkerrors.Wrap(types.ErrMigrationFailed, "requires migrate from admin")
	}

	random := k.GetRandomSeed(ctx, ctx.BlockHeight())

	env := types.NewEnv(ctx, caller, sdk.Coins{}, contractAddress, &contractKey, random)

	adminProof := k.GetContractInfo(ctx, contractAddress).AdminProof
	admin := k.GetContractInfo(ctx, contractAddress).Admin

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
		Caller:  contractAddress,
	}

	// instantiate wasm contract
	gas := gasForContract(ctx)

	response, newContractKey, proof, gasUsed, err := k.wasmer.Migrate(newCodeInfo.CodeHash, env, msg, prefixStore, cosmwasmAPI, querier, gasMeter(ctx), gas, verificationInfo, admin, adminProof)
	consumeGas(ctx, gasUsed)

	// update contract key with new one
	k.SetContractKey(ctx, contractAddress, &types.ContractKey{
		Key: newContractKey,
		Original: &types.ContractKeyWithProof{
			Key:   contractKey.Key,
			Proof: proof,
		},
	})

	if err != nil {
		var result []byte
		switch res := response.(type) { //nolint:gocritic
		case v1wasmTypes.DataWithInternalReplyInfo:
			result, err = json.Marshal(res)
			if err != nil {
				return nil, sdkerrors.Wrap(err, "couldn't marshal internal reply info")
			}
		}

		return result, sdkerrors.Wrap(types.ErrExecuteFailed, err.Error())
	}

	// delete old secondary index entry
	k.removeFromContractCodeSecondaryIndex(ctx, contractAddress, k.getLastContractHistoryEntry(ctx, contractAddress))
	// persist migration updates
	historyEntry := contractInfo.AddMigration(ctx, newCodeID, msg)
	k.appendToContractHistory(ctx, contractAddress, historyEntry)
	k.addToContractCodeSecondaryIndex(ctx, contractAddress, historyEntry)
	k.setContractInfo(ctx, contractAddress, &contractInfo)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeMigrate,
		sdk.NewAttribute(types.AttributeKeyCodeID, strconv.FormatUint(newCodeID, 10)),
		sdk.NewAttribute(types.AttributeKeyContractAddr, contractAddress.String()),
	))

	switch res := response.(type) {
	case *v010wasmTypes.HandleResponse:
		subMessages, err := V010MsgsToV1SubMsgs(contractAddress.String(), res.Messages)
		if err != nil {
			return nil, sdkerrors.Wrap(err, "couldn't convert v0.10 messages to v1 messages")
		}

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, subMessages, res.Log, []v1wasmTypes.Event{}, res.Data, msg, verificationInfo, wasmTypes.CosmosMsgVersionV010)
		if err != nil {
			return nil, sdkerrors.Wrap(err, "dispatch")
		}

		return data, nil
	case *v1wasmTypes.Response:
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeMigrate,
			sdk.NewAttribute(types.AttributeKeyContractAddr, contractAddress.String()),
			sdk.NewAttribute(types.AttributeKeyCodeID, strconv.FormatUint(newCodeID, 10)),
		))

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, res.Messages, res.Attributes, res.Events, res.Data, msg, verificationInfo, wasmTypes.CosmosMsgVersionV1)
		if err != nil {
			return nil, sdkerrors.Wrap(err, "dispatch")
		}

		return data, nil
	default:
		return nil, sdkerrors.Wrap(types.ErrExecuteFailed, fmt.Sprintf("cannot detect response type: %+v", res))
	}
}

// getLastContractHistoryEntry returns the last element from history. To be used internally only as it panics when none exists
func (k Keeper) getLastContractHistoryEntry(ctx sdk.Context, contractAddr sdk.AccAddress) types.ContractCodeHistoryEntry {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.GetContractCodeHistoryElementPrefix(contractAddr))
	iter := prefixStore.ReverseIterator(nil, nil)
	defer iter.Close()

	var r types.ContractCodeHistoryEntry
	if !iter.Valid() {
		// all contracts have a history
		panic(fmt.Sprintf("no history for %s", contractAddr.String()))
	}
	k.cdc.MustUnmarshal(iter.Value(), &r)
	return r
}

// removeFromContractCodeSecondaryIndex removes element to the index for contracts-by-codeid queries
func (k Keeper) removeFromContractCodeSecondaryIndex(ctx sdk.Context, contractAddress sdk.AccAddress, entry types.ContractCodeHistoryEntry) {
	ctx.KVStore(k.storeKey).Delete(types.GetContractByCreatedSecondaryIndexKey(contractAddress, entry))
}

func (k Keeper) appendToContractHistory(ctx sdk.Context, contractAddr sdk.AccAddress, newEntries ...types.ContractCodeHistoryEntry) {
	store := ctx.KVStore(k.storeKey)
	// find last element position
	var pos uint64
	prefixStore := prefix.NewStore(store, types.GetContractCodeHistoryElementPrefix(contractAddr))
	iter := prefixStore.ReverseIterator(nil, nil)
	defer iter.Close()

	if iter.Valid() {
		pos = sdk.BigEndianToUint64(iter.Key())
	}
	// then store with incrementing position
	for _, e := range newEntries {
		pos++
		key := types.GetContractCodeHistoryElementKey(contractAddr, pos)
		store.Set(key, k.cdc.MustMarshal(&e)) //nolint:gosec
	}
}

func (k Keeper) GetContractHistory(ctx sdk.Context, contractAddr sdk.AccAddress) []types.ContractCodeHistoryEntry {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.GetContractCodeHistoryElementPrefix(contractAddr))
	r := make([]types.ContractCodeHistoryEntry, 0)
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var e types.ContractCodeHistoryEntry
		k.cdc.MustUnmarshal(iter.Value(), &e)
		r = append(r, e)
	}
	return r
}

// addToContractCodeSecondaryIndex adds element to the index for contracts-by-codeid queries
func (k Keeper) addToContractCodeSecondaryIndex(ctx sdk.Context, contractAddress sdk.AccAddress, entry types.ContractCodeHistoryEntry) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetContractByCreatedSecondaryIndexKey(contractAddress, entry), []byte{})
}

func (k Keeper) GetStoreKey() sdk.StoreKey {
	return k.storeKey
}
