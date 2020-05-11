package keeper

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/api"
	ra "github.com/enigmampc/EnigmaBlockchain/x/registration/remote_attestation"
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/EnigmaBlockchain/x/registration/internal/types"
)

// User struct which contains a name
// a type and a list of social links
type SeedConfig struct {
	PublicKey    string `json:"pk"`
	EncryptedKey string `json:"encKey"`
}

func (c SeedConfig) decode() ([]byte, []byte, error) {
	enc, err := hex.DecodeString(c.EncryptedKey)
	if err != nil {
		return nil, nil, err
	}
	pk, err := hex.DecodeString(c.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	return pk, enc, nil
}

// Keeper will have a reference to Wasmer with it's own data directory.
type Keeper struct {
	storeKey sdk.StoreKey
	cdc      *codec.Codec

	router sdk.Router
}

func SgxMode() string {
	sgx := os.Getenv("SGX_MODE")
	if sgx == "" {
		sgx = "HW"
	}

	return sgx
}

// NewKeeper creates a new contract Keeper instance
func NewKeeper(cdc *codec.Codec, storeKey sdk.StoreKey, router sdk.Router, homeDir string, bootstrap bool) Keeper {

	if SgxMode() == "HW" {
		// validate attestation

	}

	if !bootstrap {

		seedPath := filepath.Join(homeDir, "seed.json")

		if !fileExists(seedPath) {
			panic(sdkerrors.Wrap(types.ErrSeedInitFailed, "Seed configuration not found. Did you initialize the node?"))
		}

		// get PK from CLI
		// get encrypted master key
		byteValue, err := getFile(seedPath)

		var seedCfg SeedConfig

		err = json.Unmarshal(byteValue, &seedCfg)
		if err != nil {
			panic(sdkerrors.Wrap(types.ErrSeedInitFailed, err.Error()))
		}

		err = validateSeedParams(seedCfg)
		if err != nil {
			panic(sdkerrors.Wrap(types.ErrSeedInitFailed, err.Error()))
		}

		pk, enc, err := seedCfg.decode()
		if err != nil {
			panic(sdkerrors.Wrap(types.ErrSeedInitFailed, err.Error()))
		}

		_, err = api.InitSeed(pk, enc)
		if err != nil {
			panic(sdkerrors.Wrap(types.ErrSeedInitFailed, err.Error()))
		}
	}

	return Keeper{
		storeKey: storeKey,
		cdc:      cdc,
		router:   router,
	}
}

// Create uploads and compiles a WASM contract, returning a short identifier for the contract
func (k Keeper) AuthenticateNode(ctx sdk.Context, certificate ra.Certificate) ([]byte, error) {
	fmt.Println("AuthenticateNode")
	var encSeed []byte

	if isSimulationMode(ctx) {
		// any sha256 hash is good enough
		encSeed = make([]byte, 32)
	} else {

		publicKey, err := ra.VerifyRaCert(certificate)
		if err != nil {
			return nil, sdkerrors.Wrap(types.ErrAuthenticateFailed, err.Error())
		}

		isAuth, err := k.isNodeAuthenticated(ctx, publicKey)
		if err != nil {
			return nil, sdkerrors.Wrap(types.ErrAuthenticateFailed, err.Error())
		}
		fmt.Println("After isNodeAuthenticated")
		if isAuth {
			return k.getRegistrationInfo(ctx, publicKey).EncryptedSeed, nil
		}
		fmt.Println("After getRegistrationInfo")
		encSeed, err = api.GetEncryptedSeed(certificate)
		fmt.Println("After GetEncryptedSeed")
		if err != nil {
			// return 0, sdkerrors.Wrap(err, "cosmwasm create")
			return nil, sdkerrors.Wrap(types.ErrAuthenticateFailed, err.Error())
		}
		fmt.Println("Woohoo")
	}

	regInfo := types.RegistrationNodeInfo{
		Certificate:   certificate,
		EncryptedSeed: encSeed,
	}
	k.setRegistrationInfo(ctx, regInfo)

	return encSeed, nil
}

// returns true when simulation mode used by gas=auto queries
func isSimulationMode(ctx sdk.Context) bool {
	return ctx.GasMeter().Limit() == 0 && ctx.BlockHeight() != 0
}

//func (k Keeper) dispatchMessages(ctx sdk.Context, contract exported.Account, msgs []wasmTypes.CosmosMsg) error {
//	for _, msg := range msgs {
//		if err := k.dispatchMessage(ctx, contract, msg); err != nil {
//			return err
//		}
//	}
//	return nil
//}
//
//func (k Keeper) dispatchMessage(ctx sdk.Context, contract exported.Account, msg wasmTypes.CosmosMsg) error {
//	// maybe use this instead for the arg?
//	contractAddr := contract.GetAddress()
//	if msg.Send != nil {
//		return k.sendTokens(ctx, contractAddr, msg.Send.FromAddress, msg.Send.ToAddress, msg.Send.Amount)
//	} else if msg.Contract != nil {
//		targetAddr, stderr := sdk.AccAddressFromBech32(msg.Contract.ContractAddr)
//		if stderr != nil {
//			return sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, msg.Contract.ContractAddr)
//		}
//		sentFunds, err := convertWasmCoinToSdkCoin(msg.Contract.Send)
//		if err != nil {
//			return err
//		}
//		_, err = k.Execute(ctx, targetAddr, contractAddr, msg.Contract.Msg, sentFunds)
//		return err // may be nil
//	} else if msg.Opaque != nil {
//		msg, err := ParseOpaqueMsg(k.cdc, msg.Opaque)
//		if err != nil {
//			return err
//		}
//		return k.handleSdkMessage(ctx, contractAddr, msg)
//	}
//	// what is it?
//	panic(fmt.Sprintf("Unknown CosmosMsg: %#v", msg))
//}

func (k Keeper) handleSdkMessage(ctx sdk.Context, contractAddr sdk.Address, msg sdk.Msg) error {
	// make sure this account can send it
	for _, acct := range msg.GetSigners() {
		if !acct.Equals(contractAddr) {
			return sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "contract doesn't have permission")
		}
	}

	// find the handler and execute it
	h := k.router.Route(ctx, msg.Route())
	if h == nil {
		return sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, msg.Route())
	}
	res, err := h(ctx, msg)
	if err != nil {
		return err
	}
	// redispatch all events, (type sdk.EventTypeMessage will be filtered out in the handler)
	ctx.EventManager().EmitEvents(res.Events)

	return nil
}

func validateSeedParams(config SeedConfig) error {
	if len(config.PublicKey) != types.PublicKeyLength || !isHexString(config.PublicKey) {
		return sdkerrors.Wrap(types.ErrSeedValidationParams, "Invalid parameter `public key` in seed parameters. Did you initialize the node?")
	}
	if len(config.EncryptedKey) != types.EncryptedKeyLength || !isHexString(config.EncryptedKey) {
		return sdkerrors.Wrap(types.ErrSeedValidationParams, "Invalid parameter: `seed` in seed parameters. Did you initialize the node?")
	}
	return nil
}

func isHexString(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}
