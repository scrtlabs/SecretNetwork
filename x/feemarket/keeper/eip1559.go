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
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
)

// CalculateBaseFee calculates the base fee for the current block. This is only calculated once per
// block during BeginBlock. If the NoBaseFee parameter is enabled or below activation height, this function returns nil.
// NOTE: This code is inspired from the go-ethereum EIP1559 implementation and adapted to Cosmos SDK-based
// chains. For the canonical code refer to: https://github.com/ethereum/go-ethereum/blob/master/consensus/misc/eip1559.go
func (k Keeper) CalculateBaseFee(ctx sdk.Context) *big.Int {
	params := k.GetParams(ctx)

	// Ignore the calculation if not enabled
	if !params.IsBaseFeeEnabled(ctx.BlockHeight()) {
		return nil
	}

	consParams := ctx.ConsensusParams()

	// If the current block is the first EIP-1559 block, return the base fee
	// defined in the parameters (DefaultBaseFee if it hasn't been changed by
	// governance).
	if ctx.BlockHeight() == params.EnableHeight {
		return params.BaseFee.BigInt()
	}

	// get the block gas used and the base fee values for the parent block.
	// NOTE: this is not the parent's base fee but the current block's base fee,
	// as it is retrieved from the transient store, which is committed to the
	// persistent KVStore after EndBlock (ABCI Commit).
	parentBaseFee := params.BaseFee.BigInt()
	if parentBaseFee == nil {
		return nil
	}

	parentGasWanted := k.GetBlockGasWanted(ctx)
	gasLimit := new(big.Int).SetUint64(math.MaxUint64)

	// NOTE: a MaxGas equal to -1 means that block gas is unlimited
	if consParams != nil && consParams.Block != nil && consParams.Block.MaxGas > -1 {
		gasLimit = big.NewInt(consParams.Block.MaxGas)
	}

	// CONTRACT: ElasticityMultiplier cannot be 0 as it's checked in the params
	// validation
	parentGasTargetBig := new(big.Int).Div(gasLimit, new(big.Int).SetUint64(uint64(params.ElasticityMultiplier)))
	if !parentGasTargetBig.IsUint64() {
		return nil
	}

	parentGasTarget := parentGasTargetBig.Uint64()
	baseFeeChangeDenominator := new(big.Int).SetUint64(uint64(params.BaseFeeChangeDenominator))

	// If gas used == gas target, base fee remains unchanged
	if parentGasWanted == parentGasTarget {
		return parentBaseFee
	}

	var (
		num   = new(big.Int)
		denom = new(big.Int)
	)

	// If gas used > gas target, base fee increases
	if parentGasWanted > parentGasTarget {
		// If the parent block used more gas than its target, the baseFee should increase.
		// max(1, parentBaseFee * gasUsedDelta / parentGasTarget / baseFeeChangeDenominator)
		num.SetUint64(parentGasWanted - parentGasTarget)
		num.Mul(num, parentBaseFee)
		num.Div(num, denom.SetUint64(parentGasTarget))
		num.Div(num, baseFeeChangeDenominator)
		baseFeeDelta := math.BigMax(num, common.Big1)

		return num.Add(parentBaseFee, baseFeeDelta)
	}

	// If gas used < gas target, base fee decreases
	if parentGasWanted < parentGasTarget {
		// Otherwise if the parent block used less gas than its target, the baseFee should decrease.
		// max(0, parentBaseFee * gasUsedDelta / parentGasTarget / baseFeeChangeDenominator)
		num.SetUint64(parentGasTarget - parentGasWanted)
		num.Mul(num, parentBaseFee)
		num.Div(num, denom.SetUint64(parentGasTarget))
		num.Div(num, baseFeeChangeDenominator)
		baseFee := num.Sub(parentBaseFee, num)

		return math.BigMax(baseFee, params.MinGasPrice.TruncateInt().BigInt())
	}

	return nil
}
