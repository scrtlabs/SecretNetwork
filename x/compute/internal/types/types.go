package types

import (
	"encoding/base64"

	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdktxsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	wasmTypes "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	"github.com/spf13/cast"
)

const defaultLRUCacheSize = uint64(0)
const defaultEnclaveLRUCacheSize = uint8(0) // can safely go up to 15
const defaultQueryGasLimit = uint64(10_000_000)

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
	/*
		if err := c.InstantiateConfig.ValidateBasic(); err != nil {
			return sdkerrors.Wrap(err, "instantiate config")
		}
	*/
	return nil
}

// NewCodeInfo fills a new Contract struct
func NewCodeInfo(codeHash []byte, creator sdk.AccAddress, source string, builder string /* , instantiatePermission AccessConfig */) CodeInfo {
	return CodeInfo{
		CodeHash: codeHash,
		Creator:  creator,
		Source:   source,
		Builder:  builder,
		// InstantiateConfig: instantiatePermission,
	}
}

/*
type ContractCodeHistoryOperationType string

const (
	InitContractCodeHistoryType    ContractCodeHistoryOperationType = "Init"
	MigrateContractCodeHistoryType ContractCodeHistoryOperationType = "Migrate"
	GenesisContractCodeHistoryType ContractCodeHistoryOperationType = "Genesis"
)

var AllCodeHistoryTypes = []ContractCodeHistoryOperationType{InitContractCodeHistoryType, MigrateContractCodeHistoryType}

// ContractCodeHistoryEntry stores code updates to a contract.
type ContractCodeHistoryEntry struct {
	Operation ContractCodeHistoryOperationType `json:"operation"`
	CodeID    uint64                           `json:"code_id"`
	Updated   *AbsoluteTxPosition              `json:"updated,omitempty"`
	Msg       json.RawMessage                  `json:"msg,omitempty"`
}
*/

// NewContractInfo creates a new instance of a given WASM contract info
func NewContractInfo(codeID uint64, creator /* , admin */ sdk.AccAddress, label string, createdAt *AbsoluteTxPosition) ContractInfo {
	return ContractInfo{
		CodeID:  codeID,
		Creator: creator,
		// Admin:   admin,
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
	/*
		if c.Admin != nil {
			if err := sdk.VerifyAddressFormat(c.Admin); err != nil {
				return sdkerrors.Wrap(err, "admin")
			}
		}
	*/
	if err := validateLabel(c.Label); err != nil {
		return sdkerrors.Wrap(err, "label")
	}
	return nil
}

/*
func (c ContractInfo) InitialHistory(initMsg []byte) ContractCodeHistoryEntry {
	return ContractCodeHistoryEntry{
		Operation: InitContractCodeHistoryType,
		CodeID:    c.CodeID,
		Updated:   c.Created,
		Msg:       initMsg,
	}
}

func (c *ContractInfo) AddMigration(ctx sdk.Context, codeID uint64, msg []byte) ContractCodeHistoryEntry {
	h := ContractCodeHistoryEntry{
		Operation: MigrateContractCodeHistoryType,
		CodeID:    codeID,
		Updated:   NewAbsoluteTxPosition(ctx),
		Msg:       msg,
	}
	c.CodeID = codeID
	return h
}

// ResetFromGenesis resets contracts timestamp and history.
func (c *ContractInfo) ResetFromGenesis(ctx sdk.Context) ContractCodeHistoryEntry {
	c.Created = NewAbsoluteTxPosition(ctx)
	return ContractCodeHistoryEntry{
		Operation: GenesisContractCodeHistoryType,
		CodeID:    c.CodeID,
		Updated:   c.Created,
	}
}
*/

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
	if ctx.BlockTime().Unix() < 0 {
		panic("Block (unix) time must never be negative ")
	}
	env := wasmTypes.Env{
		Block: wasmTypes.BlockInfo{
			Height:  uint64(ctx.BlockHeight()),
			Time:    uint64(ctx.BlockTime().Unix()),
			ChainID: ctx.ChainID(),
		},
		Message: wasmTypes.MessageInfo{
			Sender:    creator.String(),
			SentFunds: NewWasmCoins(deposit),
		},
		Contract: wasmTypes.ContractInfo{
			Address: contractAddr.String(),
		},
		Key: wasmTypes.ContractKey(base64.StdEncoding.EncodeToString(contractKey)),
		Recursive: false,
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

const CustomEventType = "wasm"
const AttributeKeyContractAddr = "contract_address"

// ParseEvents converts wasm LogAttributes into an sdk.Events (with 0 or 1 elements)
func ParseEvents(logs []wasmTypes.LogAttribute, contractAddr sdk.AccAddress) sdk.Events {
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
	// each wasm invokation always returns one sdk.Event
	return sdk.Events{sdk.NewEvent(CustomEventType, attrs...)}
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
