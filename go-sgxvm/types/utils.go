package types

import "math/big"

func BytesToBig(value []byte) *big.Int {
	buffer := new(big.Int)
	buffer.SetBytes(value)
	return buffer
}
