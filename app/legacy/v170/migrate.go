package legacy

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/x/genutil/types"

	v120registration "github.com/scrtlabs/SecretNetwork/x/registration/legacy/v120"
	v170registration "github.com/scrtlabs/SecretNetwork/x/registration/legacy/v170"
)

func Migrate(appState types.AppMap, clientCtx client.Context) types.AppMap {
	legacyAminoCodec := codec.NewLegacyAmino()

	if appState[v120registration.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var registerGenState v120registration.GenesisState
		legacyAminoCodec.MustUnmarshalJSON(appState[v120registration.ModuleName], &registerGenState)

		// delete deprecated x/registration genesis state
		delete(appState, v120registration.ModuleName)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v170registration.ModuleName] = legacyAminoCodec.MustMarshalJSON(v170registration.Migrate(registerGenState))
	}

	return appState
}
