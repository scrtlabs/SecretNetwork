package keeper

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/api"
	"github.com/enigmampc/EnigmaBlockchain/x/registration/internal/types"
	ra "github.com/enigmampc/EnigmaBlockchain/x/registration/remote_attestation"
	"os"
	"path/filepath"
)

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

	if !bootstrap {
		InitializeNode(homeDir)
	}

	return Keeper{
		storeKey: storeKey,
		cdc:      cdc,
		router:   router,
	}
}

func InitializeNode(homeDir string) {

	if SgxMode() != "HW" {
		return
	}

	seedPath := filepath.Join(homeDir, types.SecretNodeCfgFolder, types.SecretNodeSeedConfig)

	if !fileExists(seedPath) {
		panic(sdkerrors.Wrap(types.ErrSeedInitFailed, "Seed configuration not found. Did you initialize the node?"))
	}

	// get PK from CLI
	// get encrypted master key
	byteValue, err := getFile(seedPath)

	var seedCfg types.SeedConfig

	err = json.Unmarshal(byteValue, &seedCfg)
	if err != nil {
		panic(sdkerrors.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	err = validateSeedParams(seedCfg)
	if err != nil {
		panic(sdkerrors.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	cert, enc, err := seedCfg.Decode()
	if err != nil {
		panic(sdkerrors.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	_, err = api.LoadSeedToEnclave(cert, enc)
	if err != nil {
		panic(sdkerrors.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	return
}

// Create uploads and compiles a WASM contract, returning a short identifier for the contract
func (k Keeper) RegisterNode(ctx sdk.Context, certificate ra.Certificate) ([]byte, error) {
	fmt.Println("RegisterNode")
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

func validateSeedParams(config types.SeedConfig) error {
	res, err := base64.StdEncoding.DecodeString(config.MasterCert)
	if err != nil {
		return err
	}

	res, err = ra.VerifyRaCert(res)
	if err != nil {
		return err
	}

	if len(config.EncryptedKey) != types.EncryptedKeyLength || !IsHexString(config.EncryptedKey) {
		return sdkerrors.Wrap(types.ErrSeedValidationParams, "Invalid parameter: `seed` in seed parameters. Did you initialize the node?")
	}
	return nil
}

func IsHexString(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}
