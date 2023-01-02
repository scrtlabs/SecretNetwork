package legacy

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/x/genutil/types"

	v120registration "github.com/scrtlabs/SecretNetwork/x/registration/legacy/v120"
	v170registration "github.com/scrtlabs/SecretNetwork/x/registration/legacy/v170"
)

// Migrate migrates exported state from v0.39 to a v0.40 genesis state.
func Migrate(appState types.AppMap, clientCtx client.Context) types.AppMap {
	v120Codec := codec.NewLegacyAmino()

	if appState[v120registration.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var registerGenState v120registration.GenesisState
		v120Codec.MustUnmarshalJSON(appState[v120registration.ModuleName], &registerGenState)

		// delete deprecated x/staking genesis state
		delete(appState, v120registration.ModuleName)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v170registration.ModuleName] = v120Codec.MustMarshalJSON(v170registration.Migrate(registerGenState))
	}

	return appState
}
