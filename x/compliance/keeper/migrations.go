package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/compliance/migrations/v1_0_3"
)

type Migrator struct {
	keeper Keeper
}

func NewMigrator(keeper Keeper) Migrator {
	return Migrator{
		keeper: keeper,
	}
}

func (m Migrator) Migrate1_0_2to1_0_3(ctx sdk.Context) error {
	return v1_0_3.MigrateStore(ctx, m.keeper)
}
