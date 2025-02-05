package keeper

import (
	"encoding/json"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
)

// InitGenesis sets supply information for genesis.
func InitGenesis(ctx sdk.Context, keeper Keeper, data types.GenesisState) {
	if data.IoMasterKey != nil && data.NodeExchMasterKey != nil {
		keeper.SetMasterKey(ctx, *data.IoMasterKey, types.MasterIoKeyId)
		keeper.SetMasterKey(ctx, *data.NodeExchMasterKey, types.MasterNodeKeyId)
		for _, storedRegInfo := range data.Registration {
			if err := keeper.SetRegistrationInfo(ctx, *storedRegInfo); err != nil {
				panic(err)
			}
		}
	} else {
		panic("Cannot start without MasterKey set")
	}
}

// ExportGenesis returns a GenesisState for a given context and keeper.
func ExportGenesis(ctx sdk.Context, keeper Keeper) *types.GenesisState {
	var genState types.GenesisState

	genState.NodeExchMasterKey = keeper.GetMasterKey(ctx, types.MasterNodeKeyId)
	genState.IoMasterKey = keeper.GetMasterKey(ctx, types.MasterIoKeyId)

	keeper.ListRegistrationInfo(
		ctx,
		func(_ []byte, regInfo types.RegistrationNodeInfo) bool {
			// unused param pubkey
			genState.Registration = append(genState.Registration, &regInfo)
			return false
		},
	)

	return &genState
}

func GetGenesisStateFromAppState(cdc codec.Codec, appState map[string]json.RawMessage) types.GenesisState {
	var genesisState types.GenesisState

	if appState[types.ModuleName] != nil {
		cdc.MustUnmarshalJSON(appState[types.ModuleName], &genesisState)
	}

	return genesisState
}
