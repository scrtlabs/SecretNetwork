package keeper

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"cosmossdk.io/log"
	"github.com/hashicorp/go-metrics"

	"github.com/cosmos/cosmos-sdk/telemetry"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"

	"cosmossdk.io/store/prefix"
	wasmtypes "github.com/scrtlabs/SecretNetwork/x/compute"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/scrtlabs/SecretNetwork/x/cron/types"
)

var (
	LabelExecuteReadySchedules   = "execute_ready_schedules"
	LabelScheduleCount           = "schedule_count"
	LabelScheduleExecutionsCount = "schedule_executions_count"

	MetricLabelSuccess      = "success"
	MetricLabelScheduleName = "schedule_name"
)

type (
	Keeper struct {
		cdc           codec.BinaryCodec
		storeKey      storetypes.StoreKey
		memKey        storetypes.StoreKey
		accountKeeper types.AccountKeeper
		WasmMsgServer types.WasmMsgServer
		authority     string
		txConfig      client.TxConfig
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey,
	memKey storetypes.StoreKey,
	accountKeeper types.AccountKeeper,
	authority string,
) *Keeper {
	return &Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		memKey:        memKey,
		accountKeeper: accountKeeper,
		authority:     authority,
		txConfig:      nil,
	}
}

// SetTxConfig injects the transaction configuration into the keeper.
func (k *Keeper) SetTxConfig(txConfig client.TxConfig) {
	k.txConfig = txConfig
}

func (k Keeper) GetAuthority() string {
	return k.authority
}

func (k *Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// ExecuteReadySchedules gets all schedules that are due for execution (with limit that is equal to Params.Limit)
// and executes messages in each one
func (k *Keeper) ExecuteReadySchedules(ctx sdk.Context, executionStage types.ExecutionStage) {
	telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), LabelExecuteReadySchedules)
	schedules := k.getSchedulesReadyForExecution(ctx, executionStage)

	for _, schedule := range schedules {
		err := k.executeSchedule(ctx, schedule)
		recordExecutedSchedule(err, schedule)
	}
}

// AddSchedule adds a new schedule to be executed every certain number of blocks, specified in the `period`.
// First schedule execution is supposed to be on `now + period` block.
func (k *Keeper) AddSchedule(
	ctx sdk.Context,
	name string,
	period uint64,
	msgs []types.MsgExecuteContract,
	executionStage types.ExecutionStage,
) error {
	if k.scheduleExists(ctx, name) {
		return fmt.Errorf("schedule already exists with name=%v", name)
	}

	schedule := types.Schedule{
		Name:   name,
		Period: period,
		Msgs:   msgs,
		// let's execute newly added schedule on `now + period` block
		LastExecuteHeight: uint64(ctx.BlockHeight()), //nolint:gosec
		ExecutionStage:    executionStage,
	}

	k.storeSchedule(ctx, schedule)
	k.changeTotalCount(ctx, 1)

	return nil
}

// RemoveSchedule removes schedule with a given `name`
func (k *Keeper) RemoveSchedule(ctx sdk.Context, name string) {
	if !k.scheduleExists(ctx, name) {
		return
	}

	k.changeTotalCount(ctx, -1)
	k.removeSchedule(ctx, name)
}

// GetSchedule returns schedule with a given `name`
func (k *Keeper) GetSchedule(ctx sdk.Context, name string) (*types.Schedule, bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)
	bzSchedule := store.Get(types.GetScheduleKey(name))
	if bzSchedule == nil {
		return nil, false
	}

	var schedule types.Schedule
	k.cdc.MustUnmarshal(bzSchedule, &schedule)
	return &schedule, true
}

// GetAllSchedules returns all schedules
func (k *Keeper) GetAllSchedules(ctx sdk.Context) []types.Schedule {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)

	res := make([]types.Schedule, 0)

	iterator := storetypes.KVStorePrefixIterator(store, []byte{})
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var schedule types.Schedule
		k.cdc.MustUnmarshal(iterator.Value(), &schedule)
		res = append(res, schedule)
	}

	return res
}

func (k *Keeper) GetScheduleCount(ctx sdk.Context) int32 {
	return k.getScheduleCount(ctx)
}

func (k *Keeper) getSchedulesReadyForExecution(ctx sdk.Context, executionStage types.ExecutionStage) []types.Schedule {
	params := k.GetParams(ctx)
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)
	count := uint64(0)

	res := make([]types.Schedule, 0)

	iterator := storetypes.KVStorePrefixIterator(store, []byte{})
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var schedule types.Schedule
		k.cdc.MustUnmarshal(iterator.Value(), &schedule)

		if k.intervalPassed(ctx, schedule) && schedule.ExecutionStage == executionStage {
			res = append(res, schedule)
			count++

			if count >= params.Limit {
				k.Logger(ctx).Info("limit of schedule executions per block reached")
				return res
			}
		}
	}

	return res
}

// executeSchedule executes all msgs in a given schedule and changes LastExecuteHeight
// if at least one msg execution fails, rollback all messages
func (k *Keeper) executeSchedule(ctx sdk.Context, schedule types.Schedule) error {
	// Update the schedule's last execution height.
	schedule.LastExecuteHeight = uint64(ctx.BlockHeight()) //nolint:gosec
	k.storeSchedule(ctx, schedule)

	// Get the module private key once.
	privKey := GetModulePrivateKey()
	pubKey := privKey.PubKey()
	senderAddr := sdk.AccAddress(pubKey.Address())

	// Retrieve the account info using the derived address.
	senderAcc := k.accountKeeper.GetAccount(ctx, senderAddr)
	if senderAcc == nil {
		return fmt.Errorf("account not found for address %s", senderAddr.String())
	}
	accountNumber := senderAcc.GetAccountNumber()
	sequence := senderAcc.GetSequence()
	chainID := ctx.ChainID()

	cacheCtx, writeFn := ctx.CacheContext()
	for idx, msg := range schedule.Msgs {
		// Convert contract address from bech32.
		contractAddr, err := sdk.AccAddressFromBech32(msg.Contract)
		if err != nil {
			ctx.Logger().Info("executeSchedule: failed to extract contract address", "err", err)
			return err
		}

		// zeroPrefix := make([]byte, 64)
		// finalMsg := append(zeroPrefix, []byte(msg.Msg)...)
		// Build the MsgExecuteContract.
		encryptedMsg, err := base64.StdEncoding.DecodeString(msg.Msg)
		if err != nil {
			ctx.Logger().Info("executeSchedule: failed to decode base64 msg", "err", err)
			return err
		}
		executeMsg := wasmtypes.MsgExecuteContract{
			Sender:           senderAddr,
			Contract:         contractAddr,
			Msg:              encryptedMsg,
			SentFunds:        sdk.NewCoins(),
			CallbackCodeHash: "",
		}

		// Create a new transaction builder using the shared txConfig.
		txBuilder := k.txConfig.NewTxBuilder()
		if err := txBuilder.SetMsgs(&executeMsg); err != nil {
			return err
		}
		// Set fee and gas (adjust as needed).
		txBuilder.SetFeeAmount(sdk.NewCoins())
		txBuilder.SetGasLimit(200000)

		// Prepare signer data using the correct account info.
		signerData := authsigning.SignerData{
			Address:       senderAddr.String(),
			ChainID:       chainID,
			AccountNumber: accountNumber,
			Sequence:      sequence,
			PubKey:        pubKey,
		}

		// // Generate the sign bytes.
		// signBytes, err := authsigning.GetSignBytesAdapter(
		// 	ctx,
		// 	k.txConfig.SignModeHandler(),
		// 	sdksigning.SignMode_SIGN_MODE_DIRECT,
		// 	signerData,
		// 	txBuilder.GetTx(),
		// )
		// if err != nil {
		// 	return err
		// }

		// // Sign the transaction using the module's private key.
		// signature, err := privKey.Sign(signBytes)
		// if err != nil {
		// 	return err
		// }
		// sigV2 := sdksigning.SignatureV2{
		// 	PubKey:   pubKey,
		// 	Data:     &sdksigning.SingleSignatureData{SignMode: sdksigning.SignMode_SIGN_MODE_DIRECT, Signature: nil},
		// 	Sequence: sequence,
		// }

		// // Attach the signature to the txBuilder.
		// if err := txBuilder.SetSignatures(sigV2); err != nil {
		// 	return err
		// }

		// // Encode the signed transaction.
		// signedTxBytes, err := k.txConfig.TxEncoder()(txBuilder.GetTx())
		// if err != nil {
		// 	return err
		// }

		// // Update the cache context with the signed transaction bytes.
		// cacheCtx = cacheCtx.WithTxBytes(signedTxBytes)

		sigData := signing.SingleSignatureData{
			SignMode:  signing.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		}

		sig := signing.SignatureV2{
			PubKey:   pubKey,
			Data:     &sigData,
			Sequence: sequence,
		}

		var sigs []signing.SignatureV2
		sigs = []signing.SignatureV2{sig}
		if err := txBuilder.SetSignatures(sigs...); err != nil {
			return err
		}

		bytesToSign, err := authsigning.GetSignBytesAdapter(ctx, k.txConfig.SignModeHandler(), signing.SignMode_SIGN_MODE_DIRECT, signerData, txBuilder.GetTx())
		if err != nil {
			return err
		}

		// Sign those bytes
		sigBytes, err := privKey.Sign(bytesToSign)
		if err != nil {
			return err
		}

		// Construct the SignatureV2 struct
		sigData = signing.SingleSignatureData{
			SignMode:  signing.SignMode_SIGN_MODE_DIRECT,
			Signature: sigBytes,
		}
		sig = signing.SignatureV2{
			PubKey:   pubKey,
			Data:     &sigData,
			Sequence: sequence,
		}

		err = txBuilder.SetSignatures(sig)
		if err != nil {
			return fmt.Errorf("unable to set signatures on payload: %w", err)
		}

		txBytes, err := k.txConfig.TxEncoder()(txBuilder.GetTx())
		if err != nil {
			return err
		}

		cacheCtx = cacheCtx.WithTxBytes(txBytes)

		// Execute the contract.
		_, err = k.WasmMsgServer.ExecuteContract(cacheCtx, &executeMsg)
		if err != nil {
			ctx.Logger().Info("executeSchedule: failed to execute contract msg",
				"schedule_name", schedule.Name,
				"msg_idx", idx,
				"msg_contract", msg.Contract,
				"msg", msg.Msg,
				"error", err,
			)
			return err
		}
	}

	// Commit changes if all messages were executed successfully.
	writeFn()
	return nil
}

func GetModulePrivateKey() cryptotypes.PrivKey {
	privKeyBase64 := "8Ke2frmnGdVPipv7+xh9jClrl5EaBb9cowSUgj5GvrY="
	privKeyBytes, err := base64.StdEncoding.DecodeString(privKeyBase64)
	if err != nil {
		fmt.Printf("failed to decode private key: %v", err)
	}
	return &secp256k1.PrivKey{Key: privKeyBytes}
}

func (k *Keeper) storeSchedule(ctx sdk.Context, schedule types.Schedule) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)

	bzSchedule := k.cdc.MustMarshal(&schedule)
	store.Set(types.GetScheduleKey(schedule.Name), bzSchedule)
}

func (k *Keeper) removeSchedule(ctx sdk.Context, name string) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)

	store.Delete(types.GetScheduleKey(name))
}

func (k *Keeper) scheduleExists(ctx sdk.Context, name string) bool {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)
	return store.Has(types.GetScheduleKey(name))
}

func (k *Keeper) intervalPassed(ctx sdk.Context, schedule types.Schedule) bool {
	return uint64(ctx.BlockHeight()) > (schedule.LastExecuteHeight + schedule.Period) //nolint:gosec
}

func (k *Keeper) changeTotalCount(ctx sdk.Context, incrementAmount int32) {
	store := ctx.KVStore(k.storeKey)
	count := k.getScheduleCount(ctx)
	newCount := types.ScheduleCount{Count: count + incrementAmount}
	bzCount := k.cdc.MustMarshal(&newCount)
	store.Set(types.ScheduleCountKey, bzCount)

	telemetry.ModuleSetGauge(types.ModuleName, float32(newCount.Count), LabelScheduleCount)
}

func (k *Keeper) getScheduleCount(ctx sdk.Context) int32 {
	store := ctx.KVStore(k.storeKey)
	bzCount := store.Get(types.ScheduleCountKey)
	if bzCount == nil {
		return 0
	}

	var count types.ScheduleCount
	k.cdc.MustUnmarshal(bzCount, &count)
	return count.Count
}

func recordExecutedSchedule(err error, schedule types.Schedule) {
	telemetry.IncrCounterWithLabels([]string{LabelScheduleExecutionsCount}, 1, []metrics.Label{
		telemetry.NewLabel(telemetry.MetricLabelNameModule, types.ModuleName),
		telemetry.NewLabel(MetricLabelSuccess, strconv.FormatBool(err == nil)),
		telemetry.NewLabel(MetricLabelScheduleName, schedule.Name),
	})
}
