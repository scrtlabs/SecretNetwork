package keeper

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	channelkeeper "github.com/cosmos/ibc-go/v8/modules/core/04-channel/keeper"
	portkeeper "github.com/cosmos/ibc-go/v8/modules/core/05-port/keeper"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types"
	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
	wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	"golang.org/x/crypto/ripemd160" //nolint:staticcheck

	"github.com/cosmos/cosmos-sdk/telemetry"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	txsigning "cosmossdk.io/x/tx/signing"
	"cosmossdk.io/x/tx/signing/aminojson"
	"github.com/cosmos/cosmos-sdk/baseapp"
	codedctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	eip191 "github.com/scrtlabs/SecretNetwork/eip191"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	sdktxsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	wasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm"

	v010wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"
	v1wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v1"

	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"

	"github.com/btcsuite/btcutil/bech32"
)

type ResponseHandler interface {
	// Handle processes the data returned by a contract invocation.
	Handle(
		ctx sdk.Context,
		contractAddr sdk.AccAddress,
		ibcPort string,
		messages []v1wasmTypes.SubMsg,
		origRspData []byte,
		ogTx []byte,
		sigInfo wasmTypes.SigInfo,
	) ([]byte, error)
}

// Keeper will have a reference to Wasmer with it's own data directory.
type Keeper struct {
	storeService     store.KVStoreService
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
	maxCallDepth  uint32
	HomeDir       string
	// authZPolicy   AuthorizationPolicy
	// paramSpace    subspace.Subspace
	LastMsgManager *baseapp.LastMsgMarkerContainer
	authority      string
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
	storeService store.KVStoreService,
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
	ics4Wrapper porttypes.ICS4Wrapper,
	msgRouter MessageRouter,
	queryRouter GRPCQueryRouter,
	homeDir string,
	wasmConfig *types.WasmConfig,
	supportedFeatures string,
	customEncoders *MessageEncoders,
	customPlugins *QueryPlugins,
	lastMsgManager *baseapp.LastMsgMarkerContainer,
	authority string,
) Keeper {
	wasmer, err := wasm.NewWasmer(filepath.Join(homeDir, "wasm"), supportedFeatures, wasmConfig.CacheSize, wasmConfig.EnclaveCacheSize, wasmConfig.InitEnclave)
	if err != nil {
		panic(err)
	}

	keeper := Keeper{
		storeService:     storeService,
		cdc:              cdc,
		legacyAmino:      legacyAmino,
		wasmer:           *wasmer,
		accountKeeper:    accountKeeper,
		bankKeeper:       bankKeeper,
		portKeeper:       portKeeper,
		capabilityKeeper: capabilityKeeper,
		messenger: NewMessageHandler(
			msgRouter,
			customEncoders,
			channelKeeper,
			ics4Wrapper,
			capabilityKeeper,
			portSource,
			cdc,
		),
		queryGasLimit:  wasmConfig.SmartQueryGasLimit,
		maxCallDepth:   types.DefaultMaxCallDepth,
		HomeDir:        homeDir,
		LastMsgManager: lastMsgManager,
		authority:      authority,
	}
	// always wrap the messenger, even if it was replaced by an option
	keeper.messenger = callDepthMessageHandler{keeper.messenger, keeper.maxCallDepth}
	keeper.queryPlugins = DefaultQueryPlugins(govKeeper, distKeeper, mintKeeper, bankKeeper, stakingKeeper, queryRouter, &keeper, channelKeeper).Merge(customPlugins)

	return keeper
}

func DecodeAddr(addr string) (error, []byte) {
	// Decode Bech32
	_, data, err := bech32.Decode(addr)
	if err != nil {
		return err, nil
	}

	// Convert from Bech32 (5-bit) to 8-bit binary
	decodedBytes, err := bech32.ConvertBits(data, 5, 8, false)
	return err, decodedBytes
}

type KVPair struct {
	Key   []byte
	Value []byte
}

func (k Keeper) ReadSingleChunk(buf *bytes.Buffer) ([]byte, error) {
	var chunkSize uint32
	err := binary.Read(buf, binary.LittleEndian, &chunkSize)
	if err != nil {
		return nil, err
	}

	// Ensure there’s enough data for the chunk
	if int(chunkSize) > buf.Len() {
		return nil, errors.New("data underflow")
	}

	// Read the chunk data
	chunk := make([]byte, chunkSize)
	_, err = buf.Read(chunk)
	if err != nil {
		return nil, err
	}

	return chunk, nil
}

func (k Keeper) RotateStoreFlush(all_data *bytes.Buffer, kvs *[]KVPair) error {
	_, err := api.RotateStore(all_data.Bytes())
	if err != nil {
		return err
	}

	_, err = k.ReadSingleChunk(all_data)
	if err != nil {
		return err
	}

	// parse the result
	for all_data.Len() != 0 {

		key, err := k.ReadSingleChunk(all_data)
		if err != nil {
			return err
		}

		val, err := k.ReadSingleChunk(all_data)
		if err != nil {
			return err
		}

		*kvs = append(*kvs, KVPair{Key: key, Value: val})
	}

	all_data.Reset()
	return nil
}

func (k Keeper) RotateContractsStore(ctx sdk.Context) error {
	store := k.storeService.OpenKVStore(ctx)
	k.IterateContractInfo(ctx, func(addr sdk.AccAddress, info types.ContractInfo, _ types.ContractCustomInfo) bool {
		fmt.Println("********* Contracts info *********")

		fmt.Printf("Contract Address: %s\n", addr.String())
		fmt.Printf("Contract Address B: %x\n", addr.Bytes())

		_, addrB := DecodeAddr(addr.String())
		if addrB != nil {
			fmt.Printf("Hex Address: %s\n", hex.EncodeToString(addrB))
		}

		fmt.Printf("Contract Info: %+v\n", info)

		prefixKey := append([]byte{0x03}, addrB...)

		iterator, _ := store.Iterator(prefixKey, nil)
		defer iterator.Close()

		all_data := new(bytes.Buffer)
		var kvs []KVPair

		contractKey, _ := k.GetContractKey(ctx, addr)
		og_key := contractKey.OgContractKey

		for ; iterator.Valid(); iterator.Next() {
			key := iterator.Key()
			if !bytes.HasPrefix(key, prefixKey) {
				break
			}
			value := iterator.Value()
			// fmt.Printf("Key: %X, Value: %X\n", key, value)

			if all_data.Len() > 1024*1024*20 {
				k.RotateStoreFlush(all_data, &kvs)
			}

			if all_data.Len() == 0 {
				binary.Write(all_data, binary.LittleEndian, uint32(len(og_key)))
				all_data.Write(og_key)
			}

			binary.Write(all_data, binary.LittleEndian, uint32(len(key)))
			all_data.Write(key)
			binary.Write(all_data, binary.LittleEndian, uint32(len(value)))
			all_data.Write(value)
		}

		// all_data.Bytes()

		if all_data.Len() != 0 {
			k.RotateStoreFlush(all_data, &kvs)
		}

		// fmt.Println("----------- result -----------")
		// fmt.Printf("K%X\n", all_data)

		for _, kv := range kvs {
			store.Set(kv.Key, kv.Value)
			//	fmt.Printf("Key: %X, Value: %X\n", kv.Key, kv.Value)
		}
		// fmt.Println("----------------------")

		return false
	})

	return nil
}

func (k Keeper) SetValidatorSetEvidence(ctx sdk.Context) error {
	store := k.storeService.OpenKVStore(ctx)
	validator_set_evidence, err := store.Get(types.ValidatorSetEvidencePrefix)
	if err == nil {
		_ = api.SubmitValidatorSetEvidence(validator_set_evidence)
	}
	return nil
}

func (k Keeper) GetLastMsgMarkerContainer() *baseapp.LastMsgMarkerContainer {
	return k.LastMsgManager
}

// Create uploads and compiles a WASM contract, returning a short identifier for the contract
func (k Keeper) Create(ctx sdk.Context, creator sdk.AccAddress, wasmCode []byte, source string, builder string) (codeID uint64, err error) {
	wasmCode, err = uncompress(wasmCode)
	if err != nil {
		return 0, errorsmod.Wrap(types.ErrCreateFailed, err.Error())
	}
	params := k.GetParams(ctx)
	if uint64(len(wasmCode)) > params.MaxContractSize {
		return 0, types.ErrExceedMaxContractSize
	}
	ctx.GasMeter().ConsumeGas(uint64(params.CompileCost.MulInt64(int64(len(wasmCode))).RoundInt64()), "Compiling WASM Bytecode")

	codeHash, err := k.wasmer.Create(wasmCode)
	if err != nil {
		return 0, errorsmod.Wrap(types.ErrCreateFailed, err.Error())
	}
	store := k.storeService.OpenKVStore(ctx)
	codeID = k.autoIncrementID(ctx, types.KeyLastCodeID)

	codeInfo := types.NewCodeInfo(codeHash, creator, source, builder)
	// 0x01 | codeID (uint64) -> ContractInfo
	err = store.Set(types.GetCodeKey(codeID), k.cdc.MustMarshal(&codeInfo))
	if err != nil {
		ctx.Logger().Error("set code key", "store", err.Error())
	}

	return codeID, nil
}

func (k Keeper) GetStoreService() store.KVStoreService {
	return k.storeService
}

func (k Keeper) importCode(ctx sdk.Context, codeID uint64, codeInfo types.CodeInfo, wasmCode []byte) error {
	wasmCode, err := uncompress(wasmCode)
	if err != nil {
		return errorsmod.Wrap(types.ErrCreateFailed, err.Error())
	}
	newCodeHash, err := k.wasmer.Create(wasmCode)
	if err != nil {
		return errorsmod.Wrap(types.ErrCreateFailed, err.Error())
	}
	if !bytes.Equal(codeInfo.CodeHash, newCodeHash) {
		return errorsmod.Wrap(types.ErrInvalid, "code hashes not same")
	}

	store := k.storeService.OpenKVStore(ctx)
	key := types.GetCodeKey(codeID)
	has, err := store.Has(key)
	if err != nil {
		return err
	}
	if has {
		return errorsmod.Wrapf(types.ErrDuplicate, "duplicate code: %d", codeID)
	}
	// 0x01 | codeID (uint64) -> ContractInfo
	err = store.Set(key, k.cdc.MustMarshal(&codeInfo))
	if err != nil {
		ctx.Logger().Error("store codeId -> codeInfo", "store", err.Error())
	}
	return nil
}

func (k Keeper) GetTxInfo(ctx sdk.Context, sender sdk.AccAddress) ([]byte, sdktxsigning.SignMode, []byte, []byte, []byte, error) {
	var rawTx sdktx.TxRaw
	var parsedTx sdktx.Tx
	err := k.cdc.Unmarshal(ctx.TxBytes(), &parsedTx)
	if err != nil {
		if strings.Contains(err.Error(), "no concrete type registered for type URL /ibc") {
			// We're here because the tx is an IBC tx, and the IBC module doesn't support Amino encoding.
			// It fails decoding IBC messages with the error "no concrete type registered for type URL /ibc.core.channel.v1.MsgChannelOpenInit against interface *types.Msg".
			// "concrete type" is used to refer to the mapping between the Go struct and the Amino type string (e.g. "cosmos-sdk/MsgSend")
			// Therefore we'll manually rebuild the tx without parsing the body, as parsing it is unnecessary here anyway.
			//
			// NOTE: This does not support multisigned IBC txs (if that's even a thing).

			err := k.cdc.Unmarshal(ctx.TxBytes(), &rawTx)
			if err != nil {
				return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to decode raw transaction from bytes: %s", err.Error()))
			}

			var txAuthInfo sdktx.AuthInfo
			err = k.cdc.Unmarshal(rawTx.AuthInfoBytes, &txAuthInfo)
			if err != nil {
				return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to decode transaction auth info from bytes: %s", err.Error()))
			}

			parsedTx = sdktx.Tx{
				Body:       nil, // parsing rawTx.BodyBytes is the reason for the error, and it isn't used anyway
				AuthInfo:   &txAuthInfo,
				Signatures: rawTx.Signatures,
			}
		} else {
			return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to decode transaction from bytes: %s", err.Error()))
		}
	}

	tx := authtx.WrapTx(&parsedTx).GetTx()

	pubKeys, err := tx.GetPubKeys()
	if err != nil {
		return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to get public keys for instantiate: %s", err.Error()))
	}

	pkIndex := -1
	if sender == nil || sender.Equals(types.ZeroSender) {
		// We are in a situation where the contract gets a null msg.sender,
		// however we still need to get the sign bytes for verification against the wasm input msg inside the enclave.
		// There can be multiple signers on the tx, for example one can be the msg.sender and the another can be the gas fee payer. Another example is if this tx also contains MsgMultiSend which supports multiple msg.senders thus requiring multiple signers.
		// Not sure if we should support this or if this even matters here, as we're most likely here because it's an incoming IBC tx and the signer is the relayer.
		// For now we will just take the first signer.
		// Also, because we're not decoding the tx body anymore, we can't use tx.GetSigners() here. Therefore we'll convert the pubkey into an address.

		pubkeys, err := tx.GetPubKeys()
		if err != nil {
			return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to retrieve pubkeys from tx: %s", err.Error()))
		}

		pkIndex = 0
		sender = sdk.AccAddress(pubkeys[pkIndex].Address())
	} else {
		var _signers []string // This is just used for the error message below
		for index, pubKey := range pubKeys {
			tx_pk_addr := sdk.AccAddress(pubKey.Address())
			if tx_pk_addr != nil {
				if bytes.Equal(tx_pk_addr, sender) {
					pkIndex = index
				}
			}
			_signers = append(_signers, string(tx_pk_addr))
		}
		if pkIndex == -1 {
			return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, fmt.Sprintf("[Compute] Message sender: %v is not found in the tx signer set: %v, callback signature not provided", sender, _signers))
		}
	}

	signatures, err := tx.GetSignaturesV2()
	if err != nil {
		return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to get signatures: %s", err.Error()))
	}
	var signMode sdktxsigning.SignMode
	switch signData := signatures[pkIndex].Data.(type) {
	case *sdktxsigning.SingleSignatureData:
		signMode = signData.SignMode
		if signMode == sdktxsigning.SignMode_SIGN_MODE_UNSPECIFIED {
			// Some shitness with IBC txs' internals - they are not registered properly with the app,
			// and I think that it's something in the ibc-go repo that needs to be fixed.
			// For some txs (e.g. MsgChannelOpenInit), we can unmarsal it into parsedTx
			// but signMode turns out to be SIGN_MODE_UNSPECIFIED which is not true
			// and always should be SignMode_SIGN_MODE_DIRECT (as IBC txs don't support Amino encoding)
			// which causes `modeHandler.GetSignBytes()` down the line to fail with "can't verify sign mode SIGN_MODE_UNSPECIFIED"
			// this is a stop gap solution, however we should investigate why this is happening
			// and fix the `k.cdc.Unmarshal(ctx.TxBytes(), &parsedTx)` above, which will maybe allow us to remove
			// the rawTx parsing code
			signMode = sdktxsigning.SignMode_SIGN_MODE_DIRECT
		}
	case *sdktxsigning.MultiSignatureData:
		signMode = sdktxsigning.SignMode_SIGN_MODE_LEGACY_AMINO_JSON
	}

	signerAcc, err := ante.GetSignerAcc(ctx, k.accountKeeper, sender)
	if err != nil {
		return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to retrieve account by address: %s", err.Error()))
	}

	var signBytes []byte

	if rawTx.BodyBytes != nil && rawTx.AuthInfoBytes != nil {
		signBytes, err = authtx.DirectSignBytes(rawTx.BodyBytes, rawTx.AuthInfoBytes, ctx.ChainID(), signerAcc.GetAccountNumber())
		if err != nil {
			return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to recreate sign bytes for the tx: %s", err.Error()))
		}
	} else {
		signingData := authsigning.SignerData{
			Address:       signerAcc.GetAddress().String(),
			ChainID:       ctx.ChainID(),
			AccountNumber: signerAcc.GetAccountNumber(),
			Sequence:      signerAcc.GetSequence() - 1,
			PubKey:        signerAcc.GetPubKey(),
		}
		signingOpts, err := authtx.NewDefaultSigningOptions()
		if err != nil {
			panic(err)
		}
		signingOpts.FileResolver = k.cdc.(*codec.ProtoCodec).InterfaceRegistry()

		aminoHandler := aminojson.NewSignModeHandler(aminojson.SignModeHandlerOptions{
			FileResolver: signingOpts.FileResolver,
			TypeResolver: signingOpts.TypeResolver,
		})
		eip191Handler := eip191.NewSignModeHandler(eip191.SignModeHandlerOptions{
			AminoJsonSignModeHandler: aminoHandler,
		})
		txConfigOpts := authtx.ConfigOptions{
			EnabledSignModes: authtx.DefaultSignModes,
			CustomSignModes:  [](txsigning.SignModeHandler){*eip191Handler},
		}
		txConfig, err := authtx.NewTxConfigWithOptions(
			k.cdc.(*codec.ProtoCodec),
			txConfigOpts,
		)
		if err != nil {
			panic(err)
		}
		modeHandler := txConfig.SignModeHandler()
		signBytes, err = authsigning.GetSignBytesAdapter(ctx, modeHandler, signMode, signingData, tx)
		if err != nil {
			return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, fmt.Sprintf("Unable to recreate sign bytes for the tx: %s", err.Error()))
		}
	}

	modeInfoBytes, err := sdktxsigning.SignatureDataToProto(signatures[pkIndex].Data).Marshal()
	if err != nil {
		return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, "couldn't marshal mode info")
	}

	var pkBytes []byte
	pubKey := pubKeys[pkIndex]
	anyPubKey, err := codedctypes.NewAnyWithValue(pubKey)
	if err != nil {
		return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, "couldn't turn public key into Any")
	}
	pkBytes, err = k.cdc.Marshal(anyPubKey)
	if err != nil {
		return nil, 0, nil, nil, nil, errorsmod.Wrap(types.ErrSigFailed, "couldn't marshal public key")
	}
	return signBytes, signMode, modeInfoBytes, pkBytes, parsedTx.Signatures[pkIndex], nil
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

	if msg.Bank != nil {
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
func (k Keeper) Instantiate(ctx sdk.Context, codeID uint64, creator, admin sdk.AccAddress, initMsg []byte, label string, deposit sdk.Coins, callbackSig []byte) (sdk.AccAddress, []byte, error) {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "instantiate")

	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading CosmWasm module: init")

	signBytes := []byte{}
	signMode := sdktxsigning.SignMode_SIGN_MODE_UNSPECIFIED
	modeInfoBytes := []byte{}
	pkBytes := []byte{}
	signerSig := []byte{}
	var initError error

	// If no callback signature - we should send the actual msg sender sign bytes and signature
	if callbackSig == nil {
		signBytes, signMode, modeInfoBytes, pkBytes, signerSig, initError = k.GetTxInfo(ctx, creator)
		if initError != nil {
			return nil, nil, initError
		}
	}

	sigInfo := types.NewSigInfo(ctx.TxBytes(), signBytes, signMode, modeInfoBytes, pkBytes, signerSig, callbackSig)

	// create contract address

	store := k.storeService.OpenKVStore(ctx)
	existingAddress, err := store.Get(types.GetContractLabelPrefix(label))
	if err != nil {
		return nil, nil, err
	}

	if existingAddress != nil {
		return nil, nil, errorsmod.Wrap(types.ErrAccountExists, label)
	}

	contractAddress := k.generateContractAddress(ctx, codeID, creator)
	existingAcct := k.accountKeeper.GetAccount(ctx, contractAddress)
	if existingAcct != nil {
		return nil, nil, errorsmod.Wrap(types.ErrAccountExists, existingAcct.GetAddress().String())
	}

	// deposit initial contract funds
	if !deposit.IsZero() {
		if k.bankKeeper.BlockedAddr(creator) {
			return nil, nil, sdkerrors.ErrInvalidAddress.Wrap("blocked address can not be used")
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
	bz, err := store.Get(types.GetCodeKey(codeID))
	if err != nil {
		return nil, nil, err
	}
	if bz == nil {
		return nil, nil, errorsmod.Wrap(types.ErrNotFound, "code")
	}
	var codeInfo types.CodeInfo
	k.cdc.MustUnmarshal(bz, &codeInfo)

	random := k.GetRandomSeed(ctx, ctx.BlockHeight())

	// prepare env for contract instantiate call
	env := types.NewEnv(ctx,
		creator,
		deposit,
		contractAddress,
		types.ContractKey{
			OgContractKey:           nil,
			CurrentContractKey:      nil,
			CurrentContractKeyProof: nil,
		},
		random,
	)

	// create prefixed data store
	// 0x03 | contractAddress (sdk.AccAddress)
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx)), prefixStoreKey)

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
		Caller:  contractAddress,
	}

	response, ogContractKey, adminProof, gasUsed, initError := k.wasmer.Instantiate(codeInfo.CodeHash, env, initMsg, prefixStore, cosmwasmAPI, querier, ctx.GasMeter(), gasForContract(ctx), sigInfo, admin)
	consumeGas(ctx, gasUsed)

	if initError != nil {
		switch res := response.(type) {
		case v1wasmTypes.DataWithInternalReplyInfo:
			result, jsonError := json.Marshal(res)
			if jsonError != nil {
				return nil, nil, errorsmod.Wrap(jsonError, "couldn't marshal internal reply info")
			}

			return contractAddress, result, errorsmod.Wrap(types.ErrInstantiateFailed, initError.Error())
		}

		return contractAddress, nil, errorsmod.Wrap(types.ErrInstantiateFailed, initError.Error())
	}

	switch res := response.(type) {
	case *v010wasmTypes.InitResponse:
		// emit all events from this contract itself

		// persist instance
		createdAt := types.NewAbsoluteTxPosition(ctx)
		contractInfo := types.NewContractInfo(codeID, creator, admin.String(), adminProof, label, createdAt)

		historyEntry := contractInfo.InitialHistory(initMsg)
		k.addToContractCodeSecondaryIndex(ctx, contractAddress, historyEntry)
		// k.addToContractCreatorSecondaryIndex(ctx, creator, historyEntry.Updated, contractAddress)
		k.appendToContractHistory(ctx, contractAddress, historyEntry)

		k.setContractInfo(ctx, contractAddress, &contractInfo)
		k.SetContractKey(ctx, contractAddress, &types.ContractKey{
			OgContractKey:           ogContractKey,
			CurrentContractKey:      nil,
			CurrentContractKeyProof: nil,
		})
		err := store.Set(types.GetContractLabelPrefix(label), contractAddress)
		if err != nil {
			return nil, nil, errorsmod.Wrap(err, "store.set")
		}

		subMessages, err := V010MsgsToV1SubMsgs(contractAddress.String(), res.Messages)
		if err != nil {
			return nil, nil, errorsmod.Wrap(err, "couldn't convert v0.10 messages to v1 messages")
		}

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, subMessages, res.Log, []v1wasmTypes.Event{}, res.Data, initMsg, sigInfo)
		if err != nil {
			return nil, nil, errorsmod.Wrap(err, "dispatch")
		}

		return contractAddress, data, nil
	case *v1wasmTypes.Response:
		// persist instance first
		createdAt := types.NewAbsoluteTxPosition(ctx)
		contractInfo := types.NewContractInfo(codeID, creator, admin.String(), adminProof, label, createdAt)

		// check for IBC flag
		report, err := k.wasmer.AnalyzeCode(codeInfo.CodeHash)
		if err != nil {
			return contractAddress, nil, errorsmod.Wrap(types.ErrInstantiateFailed, err.Error())
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
		// k.addToContractCreatorSecondaryIndex(ctx, creator, historyEntry.Updated, contractAddress)
		k.appendToContractHistory(ctx, contractAddress, historyEntry)

		// persist instance
		k.setContractInfo(ctx, contractAddress, &contractInfo)
		k.SetContractKey(ctx, contractAddress, &types.ContractKey{
			OgContractKey:           ogContractKey,
			CurrentContractKey:      nil,
			CurrentContractKeyProof: nil,
		})
		err = store.Set(types.GetContractLabelPrefix(label), contractAddress)
		if err != nil {
			return nil, nil, errorsmod.Wrap(err, "store.set")
		}

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, res.Messages, res.Attributes, res.Events, res.Data, initMsg, sigInfo)
		if err != nil {
			return nil, nil, errorsmod.Wrap(err, "dispatch")
		}

		return contractAddress, data, nil
	default:
		return nil, nil, errorsmod.Wrap(types.ErrInstantiateFailed, fmt.Sprintf("cannot detect response type: %+v", res))
	}
}

// Execute executes the contract instance
func (k Keeper) Execute(ctx sdk.Context, contractAddress sdk.AccAddress, caller sdk.AccAddress, msg []byte, coins sdk.Coins, callbackSig []byte, handleType wasmTypes.HandleType) (*sdk.Result, error) {
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
		signBytes, signMode, modeInfoBytes, pkBytes, signerSig, err = k.GetTxInfo(ctx, caller)
		if err != nil {
			return nil, err
		}
	}

	sigInfo := types.NewSigInfo(ctx.TxBytes(), signBytes, signMode, modeInfoBytes, pkBytes, signerSig, callbackSig)

	contractInfo, codeInfo, prefixStore, err := k.contractInstance(ctx, contractAddress)
	if err != nil {
		return nil, err
	}

	// add more funds
	if !coins.IsZero() {
		if k.bankKeeper.BlockedAddr(caller) {
			return nil, sdkerrors.ErrInvalidAddress.Wrap("blocked address can not be used")
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

	env := types.NewEnv(ctx, caller, coins, contractAddress, contractKey, random)

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
		Caller:  contractAddress,
	}

	response, gasUsed, execErr := k.wasmer.Execute(codeInfo.CodeHash, env, msg, prefixStore, cosmwasmAPI, querier, gasMeter(ctx), gasForContract(ctx), sigInfo, handleType)
	consumeGas(ctx, gasUsed)

	if execErr != nil {
		var result sdk.Result
		var jsonError error
		switch res := response.(type) {
		case v1wasmTypes.DataWithInternalReplyInfo:
			result.Data, jsonError = json.Marshal(res)
			if jsonError != nil {
				return nil, errorsmod.Wrap(jsonError, "couldn't marshal internal reply info")
			}
		}

		return &result, errorsmod.Wrap(types.ErrExecuteFailed, execErr.Error())
	}

	switch res := response.(type) {
	case *v010wasmTypes.HandleResponse:
		subMessages, err := V010MsgsToV1SubMsgs(contractAddress.String(), res.Messages)
		if err != nil {
			return nil, errorsmod.Wrap(err, "couldn't convert v0.10 messages to v1 messages")
		}

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, subMessages, res.Log, []v1wasmTypes.Event{}, res.Data, msg, sigInfo)
		if err != nil {
			return nil, errorsmod.Wrap(err, "dispatch")
		}

		return &sdk.Result{
			Data: data,
		}, nil
	case *v1wasmTypes.Response:
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeExecute,
			sdk.NewAttribute(types.AttributeKeyContractAddr, contractAddress.String()),
		))

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, res.Messages, res.Attributes, res.Events, res.Data, msg, sigInfo)
		if err != nil {
			return nil, errorsmod.Wrap(err, "dispatch")
		}

		return &sdk.Result{
			Data: data,
		}, nil
	default:
		return nil, errorsmod.Wrap(types.ErrExecuteFailed, fmt.Sprintf("cannot detect response type: %+v", res))
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
		ctx = ctx.WithGasMeter(storetypes.NewGasMeter(k.queryGasLimit))
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
		contractKey,
		[]byte{0}, /* empty because it's unused in queries */
	)
	params.QueryDepth = queryDepth

	queryResult, gasUsed, qErr := k.wasmer.Query(codeInfo.CodeHash, params, req, prefixStore, cosmwasmAPI, querier, gasMeter(ctx), gasForContract(ctx))
	consumeGas(ctx, gasUsed)

	telemetry.SetGauge(float32(gasUsed), "compute", "keeper", "query", contractAddress.String(), "gasUsed")

	if qErr != nil {
		return nil, errorsmod.Wrap(types.ErrQueryFailed, qErr.Error())
	}
	return queryResult, nil
}

func checkAndIncreaseCallDepth(ctx sdk.Context, maxCallDepth uint32) (sdk.Context, error) {
	var callDepth uint32
	if size, ok := types.CallDepth(ctx); ok {
		callDepth = size
	}

	// increase
	callDepth++

	// did we go too far?
	if callDepth > maxCallDepth {
		return sdk.Context{}, types.ErrExceedMaxCallDepth
	}

	// set updated stack size
	return types.WithCallDepth(ctx, callDepth), nil
}

// We don't use this function since we have an encrypted state. It's here for upstream compatibility
// QueryRaw returns the contract's state for give key. For a `nil` key a empty slice result is returned.
func (k Keeper) QueryRaw(ctx sdk.Context, contractAddress sdk.AccAddress, key []byte) []types.Model {
	result := make([]types.Model, 0)
	if key == nil {
		return result
	}
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx)), prefixStoreKey)

	if val := prefixStore.Get(key); val != nil {
		return append(result, types.Model{
			Key:   key,
			Value: val,
		})
	}
	return result
}

func (k Keeper) contractInstance(ctx sdk.Context, contractAddress sdk.AccAddress) (types.ContractInfo, types.CodeInfo, prefix.Store, error) {
	store := k.storeService.OpenKVStore(ctx)

	contractBz, err := store.Get(types.GetContractAddressKey(contractAddress))
	if err != nil {
		return types.ContractInfo{}, types.CodeInfo{}, prefix.Store{}, err
	}
	if contractBz == nil {
		return types.ContractInfo{}, types.CodeInfo{}, prefix.Store{}, errorsmod.Wrap(types.ErrNotFound, "contract")
	}
	var contract types.ContractInfo
	k.cdc.MustUnmarshal(contractBz, &contract)

	contractInfoBz, err := store.Get(types.GetCodeKey(contract.CodeID))
	if err != nil {
		return types.ContractInfo{}, types.CodeInfo{}, prefix.Store{}, err
	}
	if contractInfoBz == nil {
		return types.ContractInfo{}, types.CodeInfo{}, prefix.Store{}, errorsmod.Wrap(types.ErrNotFound, "contract info")
	}
	var codeInfo types.CodeInfo
	k.cdc.MustUnmarshal(contractInfoBz, &codeInfo)
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx)), prefixStoreKey)
	return contract, codeInfo, prefixStore, nil
}

func (k Keeper) GetContractKey(ctx sdk.Context, contractAddress sdk.AccAddress) (types.ContractKey, error) {
	store := k.storeService.OpenKVStore(ctx)

	var contractKey types.ContractKey
	contractKeyBz, err := store.Get(types.GetContractEnclaveKey(contractAddress))
	if err != nil {
		return types.ContractKey{}, err
	}

	if contractKeyBz == nil {
		return types.ContractKey{}, errorsmod.Wrap(types.ErrNotFound, "contract key")
	}

	err = k.cdc.Unmarshal(contractKeyBz, &contractKey)
	if err != nil {
		return contractKey, err
	}

	return contractKey, nil
}

func (k Keeper) SetContractKey(ctx sdk.Context, contractAddress sdk.AccAddress, contractKey *types.ContractKey) {
	store := k.storeService.OpenKVStore(ctx)

	contractKeyBz := k.cdc.MustMarshal(contractKey)
	err := store.Set(types.GetContractEnclaveKey(contractAddress), contractKeyBz)
	if err != nil {
		ctx.Logger().Error("SetContractKey:", err.Error())
	}
}

func (k Keeper) GetRandomSeed(ctx sdk.Context, height int64) []byte {
	store := k.storeService.OpenKVStore(ctx)

	random, err := store.Get(types.GetRandomKey(height))
	if err != nil {
		ctx.Logger().Error("GetRandomSeed:", err.Error())
		return nil
	}

	return random
}

func (k Keeper) SetRandomSeed(ctx sdk.Context, random []byte, validator_set_evidence []byte) {
	store := k.storeService.OpenKVStore(ctx)

	ctx.Logger().Info(fmt.Sprintf("Setting random: %s", hex.EncodeToString(random)))

	err := store.Set(types.GetRandomKey(ctx.BlockHeight()), random)
	if err != nil {
		ctx.Logger().Error("SetRandomSeed:", err.Error())
	}

	err = store.Set(types.ValidatorSetEvidencePrefix, validator_set_evidence)
	if err != nil {
		ctx.Logger().Error("SetRandomSeed:", err.Error())
	}
}

func (k Keeper) GetContractAddress(ctx sdk.Context, label string) sdk.AccAddress {
	store := k.storeService.OpenKVStore(ctx)

	contractAddress, err := store.Get(types.GetContractLabelPrefix(label))
	if err != nil {
		ctx.Logger().Error("GetContractAddress:", err.Error())
		return nil
	}

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
	store := k.storeService.OpenKVStore(ctx)
	var contract types.ContractInfo
	contractBz, err := store.Get(types.GetContractAddressKey(contractAddress))
	if err != nil {
		ctx.Logger().Error("GetContractInfo:", err.Error())
		return nil
	}
	if contractBz == nil {
		return nil
	}
	k.cdc.MustUnmarshal(contractBz, &contract)
	return &contract
}

func (k Keeper) containsContractInfo(ctx sdk.Context, contractAddress sdk.AccAddress) bool {
	store := k.storeService.OpenKVStore(ctx)
	has, err := store.Has(types.GetContractAddressKey(contractAddress))
	if err != nil {
		ctx.Logger().Error("containsContractInfo", err.Error())
		return false
	}
	return has
}

func (k Keeper) setContractInfo(ctx sdk.Context, contractAddress sdk.AccAddress, contract *types.ContractInfo) {
	store := k.storeService.OpenKVStore(ctx)
	err := store.Set(types.GetContractAddressKey(contractAddress), k.cdc.MustMarshal(contract))
	if err != nil {
		ctx.Logger().Error("setContractInfo:", err.Error())
	}
}

func (k Keeper) setContractCustomInfo(ctx sdk.Context, contractAddress sdk.AccAddress, contract *types.ContractCustomInfo) {
	store := k.storeService.OpenKVStore(ctx)
	k.SetContractKey(ctx, contractAddress, contract.EnclaveKey)
	// println(fmt.Sprintf("Setting enclave key: %x: %x\n", types.GetContractEnclaveKey(contractAddress), contract.EnclaveKey))
	err := store.Set(types.GetContractLabelPrefix(contract.Label), contractAddress)
	if err != nil {
		ctx.Logger().Error("setContractCustomInfo:", err.Error())
	}
	// println(fmt.Sprintf("Setting label: %x: %x\n", types.GetContractLabelPrefix(contract.Label), contractAddress))
}

func (k Keeper) IterateContractInfo(ctx sdk.Context, cb func(sdk.AccAddress, types.ContractInfo, types.ContractCustomInfo) bool) {
	prefixStore := prefix.NewStore(runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx)), types.ContractKeyPrefix)
	iter := prefixStore.Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		var contractInfo types.ContractInfo
		k.cdc.MustUnmarshal(iter.Value(), &contractInfo)

		var contractAddress sdk.AccAddress = iter.Key()

		contractKey, err := k.GetContractKey(ctx, contractAddress)
		if err != nil {
			panic(errorsmod.Wrapf(err, "failed to get contract key for %s", contractAddress.String()))
		}

		contractCustomInfo := types.ContractCustomInfo{
			EnclaveKey: &contractKey,
			Label:      contractInfo.Label,
		}

		// cb returns true to stop early
		if cb(contractAddress, contractInfo, contractCustomInfo) {
			break
		}
	}
}

func (k Keeper) GetContractState(ctx sdk.Context, contractAddress sdk.AccAddress) store.Iterator {
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx)), prefixStoreKey)
	return prefixStore.Iterator(nil, nil)
}

func (k Keeper) importContractState(ctx sdk.Context, contractAddress sdk.AccAddress, models []types.Model) error {
	prefixStoreKey := types.GetContractStorePrefixKey(contractAddress)
	prefixStore := prefix.NewStore(runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx)), prefixStoreKey)
	for _, model := range models {
		if model.Value == nil {
			model.Value = []byte{}
		}

		if prefixStore.Has(model.Key) {
			return errorsmod.Wrapf(types.ErrDuplicate, "duplicate key: %x", model.Key)
		}
		prefixStore.Set(model.Key, model.Value)

	}
	return nil
}

func (k Keeper) GetCodeInfo(ctx sdk.Context, codeID uint64) (types.CodeInfo, error) {
	store := k.storeService.OpenKVStore(ctx)
	var codeInfo types.CodeInfo
	codeInfoBz, err := store.Get(types.GetCodeKey(codeID))
	if err != nil {
		return types.CodeInfo{}, err
	}
	if codeInfoBz == nil {
		return types.CodeInfo{}, fmt.Errorf("failed to get code info for code id %d", codeID)
	}
	k.cdc.MustUnmarshal(codeInfoBz, &codeInfo)
	return codeInfo, nil
}

func (k Keeper) containsCodeInfo(ctx sdk.Context, codeID uint64) bool {
	store := k.storeService.OpenKVStore(ctx)
	has, err := store.Has(types.GetCodeKey(codeID))
	if err != nil {
		ctx.Logger().Error("containsCodeInfo:", err.Error())
		return false
	}
	return has
}

func (k Keeper) IterateCodeInfos(ctx sdk.Context, cb func(uint64, types.CodeInfo) bool) {
	prefixStore := prefix.NewStore(runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx)), types.CodeKeyPrefix)
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
	store := k.storeService.OpenKVStore(ctx)
	var codeInfo types.CodeInfo
	codeInfoBz, err := store.Get(types.GetCodeKey(codeID))
	if err != nil {
		return nil, err
	}
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
	ogSigInfo wasmTypes.SigInfo,
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
	return responseHandler.Handle(ctx, contractAddr, ibcPort, msgs, data, ogTx, ogSigInfo)
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
		panic(storetypes.ErrorOutOfGas{Descriptor: "Wasmer function execution"})
	}
}

// generates a contract address from codeID + instanceID
func (k Keeper) generateContractAddress(ctx sdk.Context, codeID uint64, creator sdk.AccAddress) sdk.AccAddress {
	instanceID := k.autoIncrementID(ctx, types.KeyLastInstanceID)
	return contractAddress(codeID, instanceID, creator)
}

func contractAddress(codeID, instanceID uint64, creator sdk.AccAddress) sdk.AccAddress {
	contractId := codeID<<32 + instanceID
	hashSourceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(hashSourceBytes, contractId)

	hashSourceBytes = append(hashSourceBytes, creator...)

	sha := sha256.Sum256(hashSourceBytes)
	hasherRIPEMD160 := ripemd160.New()
	hasherRIPEMD160.Write(sha[:]) // does not error
	return sdk.AccAddress(hasherRIPEMD160.Sum(nil))
}

func (k Keeper) GetNextCodeID(ctx sdk.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyLastCodeID)
	if err != nil {
		ctx.Logger().Error("GetNextCodeID:", err.Error())
		return 0
	}
	id := uint64(1)
	if bz != nil {
		id = binary.BigEndian.Uint64(bz)
	}
	return id
}

func (k Keeper) autoIncrementID(ctx sdk.Context, lastIDKey []byte) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(lastIDKey)
	if err != nil {
		ctx.Logger().Error("autoIncrementID.Get:", err.Error())
		return 0
	}
	id := uint64(1)
	if bz != nil {
		id = binary.BigEndian.Uint64(bz)
	}

	bz = sdk.Uint64ToBigEndian(id + 1)
	err = store.Set(lastIDKey, bz)
	if err != nil {
		ctx.Logger().Error("autoIncrementID.Set:", err.Error())
		return 0
	}

	return id
}

// peekAutoIncrementID reads the current value without incrementing it.
func (k Keeper) peekAutoIncrementID(ctx sdk.Context, lastIDKey []byte) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(lastIDKey)
	if err != nil {
		ctx.Logger().Error("peekAutoIncrementID", err.Error())
		return 0
	}
	id := uint64(1)
	if bz != nil {
		id = binary.BigEndian.Uint64(bz)
	}
	return id
}

func (k Keeper) importAutoIncrementID(ctx sdk.Context, lastIDKey []byte, val uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	has, err := store.Has(lastIDKey)
	if err != nil {
		return err
	}
	if has {
		return errorsmod.Wrapf(types.ErrDuplicate, "autoincrement id: %s", string(lastIDKey))
	}
	bz := sdk.Uint64ToBigEndian(val)
	err = store.Set(lastIDKey, bz)
	if err != nil {
		return err
	}
	return nil
}

func (k Keeper) importContract(ctx sdk.Context, contractAddr sdk.AccAddress, customInfo *types.ContractCustomInfo, c *types.ContractInfo, state []types.Model) error {
	if !k.containsCodeInfo(ctx, c.CodeID) {
		return errorsmod.Wrapf(types.ErrNotFound, "code id: %d", c.CodeID)
	}
	if k.containsContractInfo(ctx, contractAddr) {
		return errorsmod.Wrapf(types.ErrDuplicate, "contract: %s", contractAddr)
	}

	k.setContractCustomInfo(ctx, contractAddr, customInfo)
	k.setContractInfo(ctx, contractAddr, c)
	return k.importContractState(ctx, contractAddr, state)
}

// MultipliedGasMeter wraps the GasMeter from context and multiplies all reads by out defined multiplier
type MultipiedGasMeter struct {
	originalMeter storetypes.GasMeter
}

var _ wasm.GasMeter = MultipiedGasMeter{}

func (m MultipiedGasMeter) GasConsumed() storetypes.Gas {
	return m.originalMeter.GasConsumed() * types.GasMultiplier
}

func gasMeter(ctx sdk.Context) MultipiedGasMeter {
	return MultipiedGasMeter{
		originalMeter: ctx.GasMeter(),
	}
}

type MsgDispatcher interface {
	DispatchSubmessages(ctx sdk.Context, contractAddr sdk.AccAddress, ibcPort string, msgs []v1wasmTypes.SubMsg, ogTx []byte, ogSigInfo wasmTypes.SigInfo) ([]byte, error)
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
func (h ContractResponseHandler) Handle(ctx sdk.Context, contractAddr sdk.AccAddress, ibcPort string, messages []v1wasmTypes.SubMsg, origRspData []byte, ogTx []byte, ogSigInfo wasmTypes.SigInfo) ([]byte, error) {
	result := origRspData
	switch rsp, err := h.md.DispatchSubmessages(ctx, contractAddr, ibcPort, messages, ogTx, ogSigInfo); {
	case err != nil:
		return nil, errorsmod.Wrap(err, "submessages")
	case rsp != nil:
		result = rsp
	}
	return result, nil
}

// reply is only called from keeper internal functions (dispatchSubmessages) after processing the submessage
func (k Keeper) reply(ctx sdk.Context, contractAddress sdk.AccAddress, reply v1wasmTypes.Reply, ogTx []byte, ogSigInfo wasmTypes.SigInfo) ([]byte, error) {
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

	env := types.NewEnv(ctx, contractAddress, sdk.Coins{}, contractAddress, contractKey, random)

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
		Caller:  contractAddress,
	}

	marshaledReply, err := json.Marshal(reply)
	marshaledReply = append(ogTx[0:64], marshaledReply...)

	if err != nil {
		return nil, err
	}

	response, gasUsed, execErr := k.wasmer.Execute(codeInfo.CodeHash, env, marshaledReply, prefixStore, cosmwasmAPI, querier, ctx.GasMeter(), gasForContract(ctx), ogSigInfo, wasmTypes.HandleTypeReply)
	consumeGas(ctx, gasUsed)

	if execErr != nil {
		return nil, errorsmod.Wrap(types.ErrReplyFailed, execErr.Error())
	}

	switch res := response.(type) {
	case *v010wasmTypes.HandleResponse:
		return nil, errorsmod.Wrap(types.ErrReplyFailed, fmt.Sprintf("response of reply should always be a CosmWasm v1 response type: %+v", res))
	case *v1wasmTypes.Response:
		consumeGas(ctx, gasUsed)

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeReply,
			sdk.NewAttribute(types.AttributeKeyContractAddr, contractAddress.String()),
		))

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, res.Messages, res.Attributes, res.Events, res.Data, ogTx, ogSigInfo)
		if err != nil {
			return nil, errorsmod.Wrap(types.ErrReplyFailed, err.Error())
		}

		return data, nil
	default:
		return nil, errorsmod.Wrap(types.ErrReplyFailed, fmt.Sprintf("cannot detect response type: %+v", res))
	}
}

func (k Keeper) UpdateContractAdmin(ctx sdk.Context, contractAddress, caller, newAdmin sdk.AccAddress, callbackSig []byte) error {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "update-contract-admin")
	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading CosmWasm module: update-contract-admin")

	contractInfo, codeInfo, prefixStore, err := k.contractInstance(ctx, contractAddress)
	if err != nil {
		return err
	}
	if contractInfo.Admin != caller.String() {
		return sdkerrors.ErrUnauthorized.Wrap("caller is not the admin")
	}

	signBytes := []byte{}
	signMode := sdktxsigning.SignMode_SIGN_MODE_UNSPECIFIED
	modeInfoBytes := []byte{}
	pkBytes := []byte{}
	signerSig := []byte{}

	// If no callback signature - we should send the actual msg sender sign bytes and signature
	if callbackSig == nil {
		signBytes, signMode, modeInfoBytes, pkBytes, signerSig, err = k.GetTxInfo(ctx, caller)
		if err != nil {
			return err
		}
	}

	sigInfo := types.NewSigInfo(ctx.TxBytes(), signBytes, signMode, modeInfoBytes, pkBytes, signerSig, callbackSig)

	contractKey, err := k.GetContractKey(ctx, contractAddress)
	if err != nil {
		return err
	}

	env := types.NewEnv(ctx, caller, sdk.Coins{}, contractAddress, contractKey, nil)

	currentAdminAddress, err := sdk.AccAddressFromBech32(contractInfo.Admin)
	if err != nil {
		return err
	}

	if newAdmin == nil {
		newAdmin = sdk.AccAddress{}
	}

	// prepare querier
	// TODO: this is unnecessary, get rid of this
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
		Caller:  contractAddress,
	}

	newAdminProof, updateAdminErr := k.wasmer.UpdateAdmin(codeInfo.CodeHash, env, prefixStore, cosmwasmAPI, querier, gasMeter(ctx), gasForContract(ctx), sigInfo, currentAdminAddress, contractInfo.AdminProof, newAdmin)

	if updateAdminErr != nil {
		return updateAdminErr
	}

	contractInfo.Admin = newAdmin.String()
	contractInfo.AdminProof = newAdminProof
	k.setContractInfo(ctx, contractAddress, &contractInfo)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeUpdateContractAdmin,
		sdk.NewAttribute(types.AttributeKeyContractAddr, contractAddress.String()),
		sdk.NewAttribute(types.AttributeKeyNewAdmin, newAdmin.String()),
	))

	return nil
}

func (k Keeper) Migrate(ctx sdk.Context, contractAddress sdk.AccAddress, caller sdk.AccAddress, newCodeID uint64, msg []byte, callbackSig []byte) ([]byte, error) {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "migrate")
	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading CosmWasm module: migrate")

	signBytes := []byte{}
	signMode := sdktxsigning.SignMode_SIGN_MODE_UNSPECIFIED
	modeInfoBytes := []byte{}
	pkBytes := []byte{}
	signerSig := []byte{}
	var err error

	// If no callback signature - we should send the actual msg sender sign bytes and signature
	if callbackSig == nil {
		signBytes, signMode, modeInfoBytes, pkBytes, signerSig, err = k.GetTxInfo(ctx, caller)
		if err != nil {
			return nil, err
		}
	}

	sigInfo := types.NewSigInfo(ctx.TxBytes(), signBytes, signMode, modeInfoBytes, pkBytes, signerSig, callbackSig)

	contractInfo, _, prefixStore, err := k.contractInstance(ctx, contractAddress)
	if err != nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap(errorsmod.Wrap(err, "unknown contract").Error())
	}

	newCodeInfo, err := k.GetCodeInfo(ctx, newCodeID)
	if err != nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap(errorsmod.Wrap(err, "unknown code").Error())
	}

	// check for IBC flag
	switch report, err := k.wasmer.AnalyzeCode(newCodeInfo.CodeHash); {
	case err != nil:
		return nil, errorsmod.Wrap(types.ErrMigrationFailed, err.Error())
	case !report.HasIBCEntryPoints && contractInfo.IBCPortID != "":
		// prevent update of ibc contract to non ibc contract
		return nil, errorsmod.Wrap(types.ErrMigrationFailed, "requires ibc callbacks")
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

	if contractInfo.Admin != caller.String() {
		return nil, errorsmod.Wrap(types.ErrMigrationFailed, "requires migrate from admin")
	}

	random := k.GetRandomSeed(ctx, ctx.BlockHeight())

	env := types.NewEnv(ctx, caller, sdk.Coins{}, contractAddress, contractKey, random)

	adminProof := contractInfo.AdminProof
	admin := contractInfo.Admin

	adminAddr, err := sdk.AccAddressFromBech32(admin)
	if err != nil {
		return nil, errorsmod.Wrap(types.ErrMigrationFailed, err.Error())
	}

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
		Caller:  contractAddress,
	}

	response, newContractKey, newContractKeyProof, gasUsed, migrateErr := k.wasmer.Migrate(newCodeInfo.CodeHash, env, msg, prefixStore, cosmwasmAPI, querier, gasMeter(ctx), gasForContract(ctx), sigInfo, adminAddr, adminProof)
	consumeGas(ctx, gasUsed)

	if migrateErr != nil {
		var result []byte
		var jsonError error
		switch res := response.(type) {
		case v1wasmTypes.DataWithInternalReplyInfo:
			result, jsonError = json.Marshal(res)
			if jsonError != nil {
				return nil, errorsmod.Wrap(jsonError, "couldn't marshal internal reply info")
			}
		}

		return result, errorsmod.Wrap(types.ErrMigrationFailed, migrateErr.Error())
	}

	// update contract key with new one
	k.SetContractKey(ctx, contractAddress, &types.ContractKey{
		OgContractKey:           contractKey.OgContractKey,
		CurrentContractKey:      newContractKey,
		CurrentContractKeyProof: newContractKeyProof,
	})

	// delete old secondary index entry
	k.removeFromContractCodeSecondaryIndex(ctx, contractAddress, k.getLastContractHistoryEntry(ctx, contractAddress))
	// persist migration updates
	historyEntry := contractInfo.AddMigration(ctx, newCodeID, msg)
	k.appendToContractHistory(ctx, contractAddress, historyEntry)
	k.addToContractCodeSecondaryIndex(ctx, contractAddress, historyEntry)

	contractInfo.CodeID = newCodeID
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
			return nil, errorsmod.Wrap(err, "couldn't convert v0.10 messages to v1 messages")
		}

		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, subMessages, res.Log, []v1wasmTypes.Event{}, res.Data, msg, sigInfo)
		if err != nil {
			return nil, errorsmod.Wrap(err, "dispatch")
		}

		return data, nil
	case *v1wasmTypes.Response:
		data, err := k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, res.Messages, res.Attributes, res.Events, res.Data, msg, sigInfo)
		if err != nil {
			return nil, errorsmod.Wrap(err, "dispatch")
		}

		return data, nil
	default:
		return nil, errorsmod.Wrap(types.ErrMigrationFailed, fmt.Sprintf("cannot detect response type: %+v", res))
	}
}

// getLastContractHistoryEntry returns the last element from history. To be used internally only as it panics when none exists
func (k Keeper) getLastContractHistoryEntry(ctx sdk.Context, contractAddr sdk.AccAddress) types.ContractCodeHistoryEntry {
	prefixStore := prefix.NewStore(runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx)), types.GetContractCodeHistoryElementPrefix(contractAddr))
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
	err := k.storeService.OpenKVStore(ctx).Delete(types.GetContractByCreatedSecondaryIndexKey(contractAddress, entry))
	if err != nil {
		ctx.Logger().Error("remove secondary index key", "store", err.Error())
	}
}

func (k Keeper) appendToContractHistory(ctx sdk.Context, contractAddr sdk.AccAddress, newEntries ...types.ContractCodeHistoryEntry) {
	store := k.storeService.OpenKVStore(ctx)
	// find last element position
	var pos uint64
	prefixStore := prefix.NewStore(runtime.KVStoreAdapter(store), types.GetContractCodeHistoryElementPrefix(contractAddr))
	iter := prefixStore.ReverseIterator(nil, nil)
	defer iter.Close()

	if iter.Valid() {
		pos = sdk.BigEndianToUint64(iter.Key())
	}
	// then store with incrementing position
	for _, e := range newEntries {
		pos++
		key := types.GetContractCodeHistoryElementKey(contractAddr, pos)
		err := store.Set(key, k.cdc.MustMarshal(&e))
		if err != nil {
			ctx.Logger().Error("appendToContractHistory:", err.Error())
		}
	}
}

func (k Keeper) GetContractHistory(ctx sdk.Context, contractAddr sdk.AccAddress) []types.ContractCodeHistoryEntry {
	prefixStore := prefix.NewStore(runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx)), types.GetContractCodeHistoryElementPrefix(contractAddr))
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
	store := k.storeService.OpenKVStore(ctx)
	err := store.Set(types.GetContractByCreatedSecondaryIndexKey(contractAddress, entry), []byte{})
	if err != nil {
		ctx.Logger().Error("addToContractCodeSecondaryIndex:", err.Error())
	}
}

// GetAuthority returns the x/emergencybutton module's authority.
func (k Keeper) GetAuthority() string {
	return k.authority
}
