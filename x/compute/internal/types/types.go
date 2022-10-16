package types

import (
	"encoding/base64"
	fmt "fmt"
	"strings"

	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdktxsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	wasmTypesV010 "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"
	wasmTypesV1 "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v1"
	"github.com/spf13/cast"
)

const (
	defaultLRUCacheSize        = uint64(0)
	defaultEnclaveLRUCacheSize = uint8(15)
	defaultQueryGasLimit       = uint64(10_000_000)
)

// base64 of a 64 byte key
type ContractKey string

func (m Model) ValidateBasic() error {
	if len(m.Key) == 0 {
		return sdkerrors.Wrap(ErrEmpty, "key")
	}
	return nil
}

func (c CodeInfo) ValidateBasic() error {
	if len(c.CodeHash) == 0 {
		return sdkerrors.Wrap(ErrEmpty, "code hash")
	}
	if err := sdk.VerifyAddressFormat(c.Creator); err != nil {
		return sdkerrors.Wrap(err, "creator")
	}
	if err := validateSourceURL(c.Source); err != nil {
		return sdkerrors.Wrap(err, "source")
	}
	if err := validateBuilder(c.Builder); err != nil {
		return sdkerrors.Wrap(err, "builder")
	}

	return nil
}

// NewCodeInfo fills a new Contract struct
func NewCodeInfo(codeHash []byte, creator sdk.AccAddress, source string, builder string) CodeInfo {
	return CodeInfo{
		CodeHash: codeHash,
		Creator:  creator,
		Source:   source,
		Builder:  builder,
		// InstantiateConfig: instantiatePermission,
	}
}

// NewContractInfo creates a new instance of a given WASM contract info
func NewContractInfo(codeID uint64, creator sdk.AccAddress, label string, createdAt *AbsoluteTxPosition) ContractInfo {
	return ContractInfo{
		CodeID:  codeID,
		Creator: creator,
		Label:   label,
		Created: createdAt,
	}
}

func (c *ContractInfo) ValidateBasic() error {
	if c.CodeID == 0 {
		return sdkerrors.Wrap(ErrEmpty, "code id")
	}
	if err := sdk.VerifyAddressFormat(c.Creator); err != nil {
		return sdkerrors.Wrap(err, "creator")
	}
	if err := validateLabel(c.Label); err != nil {
		return sdkerrors.Wrap(err, "label")
	}
	return nil
}

// LessThan can be used to sort
func (a *AbsoluteTxPosition) LessThan(b *AbsoluteTxPosition) bool {
	if a == nil {
		return true
	}
	if b == nil {
		return false
	}
	return a.BlockHeight < b.BlockHeight || (a.BlockHeight == b.BlockHeight && a.TxIndex < b.TxIndex)
}

// NewAbsoluteTxPosition gets a timestamp from the context
func NewAbsoluteTxPosition(ctx sdk.Context) *AbsoluteTxPosition {
	// we must safely handle nil gas meters
	var index uint64
	meter := ctx.BlockGasMeter()
	if meter != nil {
		index = meter.GasConsumed()
	}
	return &AbsoluteTxPosition{
		BlockHeight: ctx.BlockHeight(),
		TxIndex:     index,
	}
}

// NewEnv initializes the environment for a contract instance
func NewEnv(ctx sdk.Context, creator sdk.AccAddress, deposit sdk.Coins, contractAddr sdk.AccAddress, contractKey []byte) wasmTypes.Env {
	// safety checks before casting below
	if ctx.BlockHeight() < 0 {
		panic("Block height must never be negative")
	}
	nano := ctx.BlockTime().UnixNano()
	if nano < 1 {
		panic("Block (unix) time must never be empty or negative ")
	}
	env := wasmTypes.Env{
		Block: wasmTypes.BlockInfo{
			Height:  uint64(ctx.BlockHeight()),
			Time:    uint64(nano),
			ChainID: ctx.ChainID(),
		},
		Message: wasmTypes.MessageInfo{
			Sender:    creator.String(),
			SentFunds: NewWasmCoins(deposit),
		},
		Contract: wasmTypes.ContractInfo{
			Address: contractAddr.String(),
		},
		Key:        wasmTypes.ContractKey(base64.StdEncoding.EncodeToString(contractKey)),
		QueryDepth: 1,
	}

	if txCounter, ok := TXCounter(ctx); ok {
		env.Transaction = &wasmTypes.TransactionInfo{Index: txCounter}
	}
	return env
}

// NewWasmCoins translates between Cosmos SDK coins and Wasm coins
func NewWasmCoins(cosmosCoins sdk.Coins) (wasmCoins []wasmTypes.Coin) {
	for _, coin := range cosmosCoins {
		wasmCoin := wasmTypes.Coin{
			Denom:  coin.Denom,
			Amount: coin.Amount.String(),
		}
		wasmCoins = append(wasmCoins, wasmCoin)
	}
	return wasmCoins
}

// ParseEvents converts wasm LogAttributes into an sdk.Events (with 0 or 1 elements)
func ContractLogsToSdkEvents(logs []wasmTypesV010.LogAttribute, contractAddr sdk.AccAddress) sdk.Events {
	// we always tag with the contract address issuing this event
	attrs := []sdk.Attribute{sdk.NewAttribute(AttributeKeyContractAddr, contractAddr.String())}
	// append attributes from wasm to the sdk.Event
	for _, l := range logs {
		// and reserve the contract_address key for our use (not contract)
		if l.Key != AttributeKeyContractAddr {
			attr := sdk.NewAttribute(l.Key, l.Value)
			attrs = append(attrs, attr)
		}
	}

	// each wasm invocation always returns one sdk.Event
	return sdk.Events{sdk.NewEvent(CustomEventType, attrs...)}
}

const eventTypeMinLength = 2

// NewCustomEvents converts wasm events from a contract response to sdk type events
func NewCustomEvents(evts wasmTypesV1.Events, contractAddr sdk.AccAddress) (sdk.Events, error) {
	events := make(sdk.Events, 0, len(evts))
	for _, e := range evts {
		typ := strings.TrimSpace(e.Type)
		if len(typ) <= eventTypeMinLength {
			return nil, sdkerrors.Wrap(ErrInvalidEvent, fmt.Sprintf("Event type too short: '%s'", typ))
		}
		attributes, err := contractSDKEventAttributes(e.Attributes, contractAddr)
		if err != nil {
			return nil, err
		}
		events = append(events, sdk.NewEvent(fmt.Sprintf("%s%s", CustomContractEventPrefix, typ), attributes...))
	}
	return events, nil
}

// convert and add contract address issuing this event
func contractSDKEventAttributes(customAttributes []wasmTypesV010.LogAttribute, contractAddr sdk.AccAddress) ([]sdk.Attribute, error) {
	attrs := []sdk.Attribute{sdk.NewAttribute(AttributeKeyContractAddr, contractAddr.String())}
	// append attributes from wasm to the sdk.Event
	for _, l := range customAttributes {
		// ensure key and value are non-empty (and trim what is there)
		key := strings.TrimSpace(l.Key)
		if len(key) == 0 {
			return nil, sdkerrors.Wrap(ErrInvalidEvent, fmt.Sprintf("Empty attribute key. Value: %s", l.Value))
		}
		value := strings.TrimSpace(l.Value)
		// TODO: check if this is legal in the SDK - if it is, we can remove this check
		if len(value) == 0 {
			return nil, sdkerrors.Wrap(ErrInvalidEvent, fmt.Sprintf("Empty attribute value. Key: %s", key))
		}
		// and reserve all _* keys for our use (not contract)
		if strings.HasPrefix(key, AttributeReservedPrefix) {
			return nil, sdkerrors.Wrap(ErrInvalidEvent, fmt.Sprintf("Attribute key starts with reserved prefix %s: '%s'", AttributeReservedPrefix, key))
		}
		attrs = append(attrs, sdk.NewAttribute(key, value))
	}
	return attrs, nil
}

// WasmConfig is the extra config required for wasm
type WasmConfig struct {
	SmartQueryGasLimit uint64
	CacheSize          uint64
	EnclaveCacheSize   uint8
}

// DefaultWasmConfig returns the default settings for WasmConfig
func DefaultWasmConfig() *WasmConfig {
	return &WasmConfig{
		SmartQueryGasLimit: defaultQueryGasLimit,
		CacheSize:          defaultLRUCacheSize,
		EnclaveCacheSize:   defaultEnclaveLRUCacheSize,
	}
}

type SecretMsg struct {
	CodeHash []byte
	Msg      []byte
}

func NewSecretMsg(codeHash []byte, msg []byte) SecretMsg {
	return SecretMsg{
		CodeHash: codeHash,
		Msg:      msg,
	}
}

func (m SecretMsg) Serialize() []byte {
	return append(m.CodeHash, m.Msg...)
}

func NewVerificationInfo(
	signBytes []byte, signMode sdktxsigning.SignMode, modeInfo []byte, publicKey []byte, signature []byte, callbackSig []byte,
) wasmTypes.VerificationInfo {
	return wasmTypes.VerificationInfo{
		Bytes:             signBytes,
		SignMode:          signMode.String(),
		ModeInfo:          modeInfo,
		Signature:         signature,
		PublicKey:         publicKey,
		CallbackSignature: callbackSig,
	}
}

// GetConfig load config values from the app options
func GetConfig(appOpts servertypes.AppOptions) *WasmConfig {
	return &WasmConfig{
		SmartQueryGasLimit: cast.ToUint64(appOpts.Get("wasm.contract-query-gas-limit")),
		CacheSize:          cast.ToUint64(appOpts.Get("wasm.contract-memory-cache-size")),
		EnclaveCacheSize:   cast.ToUint8(appOpts.Get("wasm.contract-memory-enclave-cache-size")),
	}
}

// DefaultConfigTemplate default config template for wasm module
const DefaultConfigTemplate = `
[wasm]
# The maximum gas amount can be spent for contract query.
# The contract query will invoke contract execution vm,
# so we need to restrict the max usage to prevent DoS attack
contract-query-gas-limit = "{{ .WASMConfig.SmartQueryGasLimit }}"

# The WASM VM memory cache size in MiB not bytes
contract-memory-cache-size = "{{ .WASMConfig.CacheSize }}"

# The WASM VM memory cache size in number of cached modules. Can safely go up to 15, but not recommended for validators
contract-memory-enclave-cache-size = "{{ .WASMConfig.EnclaveCacheSize }}"
`
