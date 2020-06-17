package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	wasmTypes "github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/types"
	"github.com/stretchr/testify/require"
)

// MaskInitMsg is {}

type MaskHandleMsg struct {
	Reflect *reflectPayload `json:"reflectmsg,omitempty"`
	Change  *ownerPayload   `json:"changeowner,omitempty"`
}

type ownerPayload struct {
	Owner sdk.Address `json:"owner"`
}

type reflectPayload struct {
	Msg wasmTypes.CosmosMsg `json:"msg"`
}

func checkAccount(t *testing.T, ctx sdk.Context, accKeeper auth.AccountKeeper, addr sdk.AccAddress, expected sdk.Coins) {
	acct := accKeeper.GetAccount(ctx, addr)
	if expected == nil {
		require.Nil(t, acct)
	} else {
		require.NotNil(t, acct)
		if expected.Empty() {
			// there is confusion between nil and empty slice... let's just treat them the same
			require.True(t, acct.GetCoins().Empty())
		} else {
			require.Equal(t, acct.GetCoins(), expected)
		}
	}
}
