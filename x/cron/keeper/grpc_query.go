package keeper

import (
	"github.com/scrtlabs/SecretNetwork/x/cron/types"
)

var _ types.QueryServer = Keeper{}
