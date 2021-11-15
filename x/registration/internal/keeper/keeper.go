package keeper

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/legacy/legacytx"
	"github.com/enigmampc/SecretNetwork/x/registration/internal/types"
	ra "github.com/enigmampc/SecretNetwork/x/registration/remote_attestation"
	"path/filepath"
)

// Keeper will have a reference to Wasmer with it's own data directory.
type Keeper struct {
	storeKey sdk.StoreKey
	cdc      codec.BinaryCodec
	enclave  EnclaveInterface
	router   sdk.Router
}

// NewKeeper creates a new contract Keeper instance
func NewKeeper(cdc codec.BinaryCodec, storeKey sdk.StoreKey, router sdk.Router, enclave EnclaveInterface, homeDir string, bootstrap bool) Keeper {

	if !bootstrap {
		InitializeNode(homeDir, enclave)
	}

	return Keeper{
		storeKey: storeKey,
		cdc:      cdc,
		router:   router,
		enclave:  enclave,
	}
}

func InitializeNode(homeDir string, enclave EnclaveInterface) {

	seedPath := filepath.Join(homeDir, types.SecretNodeCfgFolder, types.SecretNodeSeedConfig)

	if !fileExists(seedPath) {
		panic(sdkerrors.Wrap(types.ErrSeedInitFailed, fmt.Sprintf("Searching for Seed configuration in path: %s was not found. Did you initialize the node?", seedPath)))
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

	_, err = enclave.LoadSeed(cert, enc)
	if err != nil {
		panic(sdkerrors.Wrap(types.ErrSeedInitFailed, err.Error()))
	}

	return
}

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
		if isAuth {
			return k.getRegistrationInfo(ctx, publicKey).EncryptedSeed, nil
		}
		encSeed, err = k.enclave.GetEncryptedSeed(certificate)
		if err != nil {
			// return 0, sdkerrors.Wrap(err, "cosmwasm create")
			return nil, sdkerrors.Wrap(types.ErrAuthenticateFailed, err.Error())
		}
	}

	regInfo := types.RegistrationNodeInfo{
		Certificate:   certificate,
		EncryptedSeed: encSeed,
	}
	k.SetRegistrationInfo(ctx, regInfo)

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

	var res *sdk.Result
	var err error
	// find the handler and execute it
	if legacyMsg, ok := msg.(legacytx.LegacyMsg); ok {
		h := k.router.Route(ctx, legacyMsg.Route())
		if h == nil {
			return sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, legacyMsg.Route())
		}
		res, err = h(ctx, msg)
		if err != nil {
			return err
		}
	}

	events := make(sdk.Events, len(res.Events))
	for i := range res.Events {
		events[i] = sdk.Event(res.Events[i])
	}

	// redispatch all events, (type sdk.EventTypeMessage will be filtered out in the handler)
	ctx.EventManager().EmitEvents(events)

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
