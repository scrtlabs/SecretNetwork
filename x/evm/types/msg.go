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
package types

import (
	"math/big"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

var (
	_ sdk.Msg    = &MsgHandleTx{}
	_ sdk.Tx     = &MsgHandleTx{}
	_ ante.GasTx = &MsgHandleTx{}
	_ sdk.Msg    = &MsgUpdateParams{}

	_ codectypes.UnpackInterfacesMessage = MsgHandleTx{}
)

// message type and route constants
const (
	// TypeMsgEthereumTx defines the type string of an Ethereum transaction
	TypeMsgEthereumTx = "ethereum_tx"
)

// NewTx returns a reference to a new Ethereum transaction message.
func NewTx(
	chainID *big.Int, nonce uint64, to *common.Address, amount *big.Int,
	gasLimit uint64, gasPrice, gasFeeCap, gasTipCap *big.Int, input []byte, accesses *ethtypes.AccessList,
	privateKey, nodePublicKey []byte,
) *MsgHandleTx {
	return newMsgHandleTx(chainID, nonce, to, amount, gasLimit, gasPrice, gasFeeCap, gasTipCap, input, accesses, privateKey, nodePublicKey)
}

func NewTxFromArgs(txArgs *EvmTxArgs, privateKey, nodePublicKey []byte) *MsgHandleTx {
	return newMsgHandleTx(
		txArgs.ChainID,
		txArgs.Nonce,
		txArgs.To,
		txArgs.Amount,
		txArgs.GasLimit,
		txArgs.GasPrice,
		txArgs.GasFeeCap,
		txArgs.GasTipCap,
		txArgs.Input,
		txArgs.Accesses,
		privateKey,
		nodePublicKey,
	)
}

// NewTxContract returns a reference to a new Ethereum transaction
// message designated for contract creation.
func NewTxContract(
	chainID *big.Int,
	nonce uint64,
	amount *big.Int,
	gasLimit uint64,
	gasPrice, gasFeeCap, gasTipCap *big.Int,
	input []byte,
	accesses *ethtypes.AccessList,
) *MsgHandleTx {
	return newMsgHandleTx(chainID, nonce, nil, amount, gasLimit, gasPrice, gasFeeCap, gasTipCap, input, accesses, nil, nil)
}

// GetSigners returns the expected signers for a MsgUpdateParams message.
func (m MsgUpdateParams) GetSigners() []sdk.AccAddress {
	//#nosec G703 -- gosec raises a warning about a non-handled error which we deliberately ignore here
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

// ValidateBasic does a sanity check of the provided data
func (m *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return errortypes.Wrap(err, "invalid authority address")
	}

	return m.Params.Validate()
}

// GetSignBytes implements the LegacyMsg interface.
func (m MsgUpdateParams) GetSignBytes() []byte {
	return sdk.MustSortJSON(AminoCdc.MustMarshalJSON(&m))
}
