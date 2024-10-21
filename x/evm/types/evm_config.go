package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
)

// EVMConfig encapsulates common parameters needed to create an EVM to execute a message
// It's mainly to reduce the number of method parameters
type EVMConfig struct {
	Params      Params
	ChainConfig *params.ChainConfig
	CoinBase    common.Address
	BaseFee     *big.Int
}
