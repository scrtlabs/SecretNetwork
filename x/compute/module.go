package compute

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	tm_type "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/scrt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
	"github.com/scrtlabs/SecretNetwork/x/compute/client/cli"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/keeper"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
	tmenclave "github.com/scrtlabs/tm-secret-enclave"
)

var (
	_ module.AppModule           = AppModule{}
	_ module.HasName             = AppModule{}
	_ module.HasGenesis          = AppModule{}
	_ module.HasServices         = AppModule{}
	_ module.HasConsensusVersion = AppModule{}
	_ module.AppModuleBasic      = AppModuleBasic{}
)

// AppModuleBasic defines the basic application module used by the compute module.
type AppModuleBasic struct{}

func (b AppModuleBasic) RegisterLegacyAminoCodec(amino *codec.LegacyAmino) {
	RegisterCodec(amino)
}

func (b AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, serveMux *runtime.ServeMux) {
	_ = types.RegisterQueryHandlerClient(context.Background(), serveMux, types.NewQueryClient(clientCtx))
}

// Name returns the compute module's name.
func (AppModuleBasic) Name() string {
	return ModuleName
}

// DefaultGenesis returns default genesis state as raw bytes for the compute
// module.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(&GenesisState{
		Params: types.DefaultParams(),
	})
}

// ValidateGenesis performs genesis state validation for the compute module.
func (AppModuleBasic) ValidateGenesis(marshaler codec.JSONCodec, _ client.TxEncodingConfig, message json.RawMessage) error {
	var data GenesisState
	err := marshaler.UnmarshalJSON(message, &data)
	if err != nil {
		return err
	}
	return ValidateGenesis(data)
}

func (b AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.GetTxCmd()
}

func (b AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

// RegisterInterfaceTypes implements InterfaceModule
func (b AppModuleBasic) RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

// ____________________________________________________________________________

// AppModule implements an application module for the compute module.
type AppModule struct {
	AppModuleBasic
	keeper Keeper
}

// NewAppModule creates a new AppModule object
func NewAppModule(keeper Keeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{},
		keeper:         keeper,
	}
}

// ConsensusVersion implements AppModule/ConsensusVersion.
func (AppModule) ConsensusVersion() uint64 { return 8 }

func (am AppModule) RegisterServices(configurator module.Configurator) {
	types.RegisterMsgServer(configurator.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(configurator.QueryServer(), NewQuerier(am.keeper))

	m := keeper.NewMigrator(am.keeper)
	err := configurator.RegisterMigration(types.ModuleName, 1, m.Migrate1to2)
	if err != nil {
		panic(err)
	}
	err = configurator.RegisterMigration(types.ModuleName, 2, m.Migrate2to3)
	if err != nil {
		panic(err)
	}

	err = configurator.RegisterMigration(types.ModuleName, 3, m.Migrate3to4)
	if err != nil {
		panic(err)
	}

	err = configurator.RegisterMigration(types.ModuleName, 4, m.Migrate4to5)
	if err != nil {
		panic(err)
	}
	err = configurator.RegisterMigration(types.ModuleName, 5, m.Migrate5to6)
	if err != nil {
		panic(err)
	}

	err = configurator.RegisterMigration(types.ModuleName, 6, m.Migrate6to7)
	if err != nil {
		panic(err)
	}

	err = configurator.RegisterMigration(types.ModuleName, 7, m.Migrate7to8)
	if err != nil {
		panic(err)
	}
}

// InitGenesis performs genesis initialization for the compute module. It returns
// no validator updates.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var genesisState GenesisState
	cdc.MustUnmarshalJSON(data, &genesisState)
	if err := InitGenesis(ctx, am.keeper, genesisState); err != nil {
		panic(err)
	}
}

// ExportGenesis returns the exported genesis state as raw bytes for the compute
// module.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := ExportGenesis(ctx, am.keeper)
	return cdc.MustMarshalJSON(gs)
}

// BeginBlock returns the begin blocker for the compute module.
func (am AppModule) BeginBlock(c context.Context) error {
	// Note: as of tendermint v0.38.0 block begin request info is no longer available
	ctx := c.(sdk.Context)
	block_header := ctx.BlockHeader()
	height := ctx.BlockHeight()

	// Initialize block-scoped execution tracking
	recorder := api.GetRecorder()
	recorder.StartBlock(height)
	// Note: Traces are fetched on-demand in replayExecution when needed,
	// since they are created during execution on the SGX node

	x2_data := scrt.UnFlatten(ctx.TxBytes())

	if block_header.EncryptedRandom != nil {
		var random, validator_set_evidence []byte

		if recorder.IsReplayMode() {
			// REPLAY MODE: Try to get from local DB first, then fetch from remote SGX node
			var found bool
			random, validator_set_evidence, found = recorder.ReplaySubmitBlockSignatures(height)
			if !found {
				// Try to fetch from remote SGX node
				client := api.GetEcallClient()
				record, err := client.FetchEcallRecord(height)
				if err != nil {
					ctx.Logger().Error("Failed to fetch ecall record from remote", "height", height, "error", err)
					return fmt.Errorf("no ecall record found for height %d: %w", height, err)
				}
				random = record.RandomSeed
				validator_set_evidence = record.ValidatorSetEvidence
				// Note: We don't cache in replay mode - we apply data only once
			}
		} else {
			// SGX MODE: Call enclave and record the result
			header, err := block_header.Marshal()
			if err != nil {
				ctx.Logger().Error("Failed to marshal block header")
				return err
			}

			commit := ctx.Commit()
			b_commit, err := commit.Marshal()
			if err != nil {
				ctx.Logger().Error("Failed to marshal commit")
				return err
			}

			tm_data := tm_type.Data{Txs: x2_data}
			data, err := tm_data.Marshal()
			if err != nil {
				ctx.Logger().Error("Failed to marshal tx data")
				return err
			}

			randomAndProof := append(block_header.EncryptedRandom.Random, block_header.EncryptedRandom.Proof...)
			random, validator_set_evidence, err = api.SubmitBlockSignatures(header, b_commit, data, randomAndProof)
			if err != nil {
				ctx.Logger().Error("Failed to submit block signatures")
				return err
			}

			// Record the result for non-SGX nodes
			if err := recorder.RecordSubmitBlockSignatures(height, random, validator_set_evidence); err != nil {
				ctx.Logger().Error("Failed to record SubmitBlockSignatures", "error", err)
				// Don't fail the block for recording errors
			}
		}

		am.keeper.SetRandomSeed(ctx, random, validator_set_evidence)
	} else {
		ctx.Logger().Debug("Non-encrypted block", "Block_hash", block_header.LastBlockId.Hash, "Height", height, "Txs", len(x2_data))
	}
	return nil
}

// EndBlock returns the end blocker for the compute module.
func (am AppModule) EndBlock(c context.Context) error {
	ctx := c.(sdk.Context)

	bytesCronMsgs, err := am.keeper.GetScheduledMsgs(ctx)
	if err != nil {
		ctx.Logger().Error("Failed to get scheduled cron msgs for end blocker", "error", err)
		// return err
	}

	cron_msgs := tm_type.Data{Txs: bytesCronMsgs}
	cron_data, err := cron_msgs.Marshal()
	if err != nil {
		ctx.Logger().Error("Failed to marshal cron_msgs")
		// return err
	}

	ctx.Logger().Info("Setting scheduled txs")
	err = tmenclave.SetScheduledTxs(cron_data)
	if err != nil {
		ctx.Logger().Error("Failed to set scheduled txs %+v", err)
		// return err
	}

	// Prune old ecall records periodically
	api.GetRecorder().PruneOldRecords(ctx.BlockHeight())

	return nil
}

// IsAppModule implements the appmodule.AppModule interface.
func (AppModule) IsAppModule() {}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}
