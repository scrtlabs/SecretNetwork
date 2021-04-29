package db

import (
	tmdb "github.com/tendermint/tm-db"
)

type DB interface {
	Get(key []byte) ([]byte, error)
	Set(key, data []byte) error
	Delete(key []byte) error
	Close() error
	Iterator(start []byte, reverse bool) Iterator
	IndexIterator(start []byte, reverse bool) Iterator
	Batch() Batch
	GetCosmosAdapter() tmdb.DB
	GetSequence(key []byte, bandwidth uint64) (Sequence, error)
	Purge(bool)
	DB() interface{}

	// Global Transaction
	SetCriticalZone()
	ReleaseCriticalZone() error
}

type Iterator interface {
	Close()
	Valid(prefix []byte) bool
	Next()
	Key() []byte
	DocumentKey() []byte
	Value() []byte
}

type Batch interface {
	Set(key, data []byte) error
	Delete(key []byte) error
	Flush() error
	FlushGT() error
	Close()
	Purge()
}

type Cache interface {
	SetInCache(key, data []byte)
}

type Sequence interface {
	Release() error
	Next() (uint64, error)
}
