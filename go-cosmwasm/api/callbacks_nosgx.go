//go:build !secretcli && nosgx

package api

import (
	dbm "github.com/cosmos/cosmos-db"
)

const GasMultiplier = 1000

type Gas = uint64

type GasMeter interface {
	GasConsumed() Gas
	ConsumeGas(amount Gas, descriptor string)
}

type KVStore interface {
	Get(key []byte) []byte
	Set(key, value []byte)
	Delete(key []byte)
	Iterator(start, end []byte) dbm.Iterator
	ReverseIterator(start, end []byte) dbm.Iterator
}

type (
	HumanAddress     func([]byte) (string, uint64, error)
	CanonicalAddress func(string) ([]byte, uint64, error)
)

type GoAPI struct {
	HumanAddress     HumanAddress
	CanonicalAddress CanonicalAddress
}
