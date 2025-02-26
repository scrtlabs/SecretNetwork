package keeper

import (
	"github.com/neutron-org/neutron/v5/x/cron/types"
)

var _ types.QueryServer = Keeper{}
