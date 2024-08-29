// Copyright 2021 Evmos Foundation
// This file is part of Evmos' Ethermint library.
//
// The Ethermint library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The Ethermint library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the Ethermint library. If not, see https://github.com/evmos/ethermint/blob/main/LICENSE
package keeper

import (
	tmtypes "github.com/cometbft/cometbft/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	evmcommontypes "github.com/scrtlabs/SecretNetwork/types"
)

// GetHashFn implements vm.GetHashFunc for Ethermint. It handles 3 cases:
//  1. The requested height matches the current height from context (and thus same epoch number)
//  2. The requested height is from an previous height from the same chain epoch
//  3. The requested height is from a height greater than the latest one
func (k Keeper) GetHashFn(ctx sdk.Context) vm.GetHashFunc {
	return func(height uint64) common.Hash {
		h, err := evmcommontypes.SafeInt64(height)
		if err != nil {
			k.Logger(ctx).Error("failed to cast height to int64", "error", err)
			return common.Hash{}
		}

		switch {
		case ctx.BlockHeight() == h:
			// Case 1: The requested height matches the one from the context so we can retrieve the header
			// hash directly from the context.
			// Note: The headerHash is only set at begin block, it will be nil in case of a query context
			headerHash := ctx.HeaderHash()
			if len(headerHash) != 0 {
				return common.BytesToHash(headerHash)
			}

			// only recompute the hash if not set (eg: checkTxState)
			contextBlockHeader := ctx.BlockHeader()
			header, err := tmtypes.HeaderFromProto(&contextBlockHeader)
			if err != nil {
				k.Logger(ctx).Error("failed to cast tendermint header from proto", "error", err)
				return common.Hash{}
			}

			headerHash = header.Hash()
			return common.BytesToHash(headerHash)

		case ctx.BlockHeight() > h:
			// Case 2: if the chain is not the current height we need to retrieve the hash from the store for the
			// current chain epoch. This only applies if the current height is greater than the requested height.
			histInfo, found := k.stakingKeeper.GetHistoricalInfo(ctx, h)
			if !found {
				k.Logger(ctx).Debug("historical info not found", "height", h)
				return common.Hash{}
			}

			header, err := tmtypes.HeaderFromProto(&histInfo.Header)
			if err != nil {
				k.Logger(ctx).Error("failed to cast tendermint header from proto", "error", err)
				return common.Hash{}
			}

			return common.BytesToHash(header.Hash())
		default:
			// Case 3: heights greater than the current one returns an empty hash.
			return common.Hash{}
		}
	}
}
