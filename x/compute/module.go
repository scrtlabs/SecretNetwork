package compute

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	wasmtypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	"github.com/scrtlabs/SecretNetwork/x/compute/client/cli"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/keeper"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
	crontypes "github.com/scrtlabs/SecretNetwork/x/cron/types"
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
func (AppModule) ConsensusVersion() uint64 { return 6 }

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

	execCronMsgs, bytesCronMsgs, err := am.keeper.GetScheduledMsgs(ctx, crontypes.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER)
	if err != nil {
		ctx.Logger().Error("Failed to get scheduled cron msgs")
		return err
	}

	cron_msgs := tm_type.Data{Txs: bytesCronMsgs}
	cron_data, err := cron_msgs.Marshal()
	if err != nil {
		ctx.Logger().Error("Failed to marshal cron_msgs")
		return err
	}

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

	x2_data := scrt.UnFlatten(ctx.TxBytes())
	tm_data := tm_type.Data{Txs: x2_data}
	data, err := tm_data.Marshal()
	if err != nil {
		ctx.Logger().Error("Failed to marshal tx data")
		return err
	}
	if block_header.EncryptedRandom != nil {
		randomAndProof := append(block_header.EncryptedRandom.Random, block_header.EncryptedRandom.Proof...)
		random, validator_set_evidence, err := api.SubmitBlockSignatures(header, b_commit, data, randomAndProof, cron_data)
		if err != nil {
			ctx.Logger().Error("Failed to submit block signatures")
			return err
		}

		for idx, msg := range execCronMsgs {
			ctx = ctx.WithTxBytes(bytesCronMsgs[idx])
			_, err := am.keeper.Execute(ctx, msg.Contract, msg.Sender, msg.Msg, msg.SentFunds, msg.CallbackSig, wasmtypes.HandleTypeExecute)
			if err != nil {
				ctx.Logger().Error("Failed to execute cron message", "error", err)
			}
		}

		am.keeper.SetRandomSeed(ctx, random, validator_set_evidence)
	} else {
		ctx.Logger().Debug("Non-encrypted block", "Block_hash", block_header.LastBlockId.Hash, "Height", ctx.BlockHeight(), "Txs", len(x2_data))
	}
	return nil
}

// EndBlock returns the end blocker for the compute module.
func (am AppModule) EndBlock(c context.Context) error {
	ctx := c.(sdk.Context)

	_, bytesCronMsgs, err := am.keeper.GetScheduledMsgs(ctx, crontypes.ExecutionStage_EXECUTION_STAGE_END_BLOCKER)
	if err != nil {
		ctx.Logger().Error("Failed to get scheduled cron msgs")
		return err
	}

	cron_msgs := tm_type.Data{Txs: bytesCronMsgs}
	cron_data, err := cron_msgs.Marshal()
	if err != nil {
		ctx.Logger().Error("Failed to marshal cron_msgs")
		return err
	}
	hash := sha256.Sum256(cron_data)

	fmt.Printf("Hash of the executed msgs in the next round: %+v\n", hex.EncodeToString(hash[:]))
	err = tmenclave.SetImplicitHash(hash[:])
	if err != nil {
		ctx.Logger().Error("Failed to set implicit hash %+v", err)
		return err
	}
	return nil
}

// IsAppModule implements the appmodule.AppModule interface.
func (AppModule) IsAppModule() {}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}
