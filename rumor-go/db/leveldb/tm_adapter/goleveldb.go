package tm_adapter

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/util"
	tmdb "github.com/tendermint/tm-db"

	"github.com/enigmampc/SecretNetwork/rumor-go/db"
)


var (
	// errBatchClosed is returned when a closed or written batch is used.
	errBatchClosed = errors.New("batch has been written or closed")

	// errKeyEmpty is returned when attempting to use an empty or nil key.
	errKeyEmpty = errors.New("key cannot be empty")

	// errValueNil is returned when attempting to set a nil value.
	errValueNil = errors.New("value cannot be nil")
)

type GoLevelDB struct {
	db db.DB
}

var _ tmdb.DB = (*GoLevelDB)(nil)

func NewCosmosAdapter(db db.DB) *GoLevelDB {
	_, ok := db.DB().(*leveldb.DB)
	if !ok {
		panic("underlying db engine mismatch. expected leveldb.")
	}
	return &GoLevelDB{db: db}
}

// Get implements DB.
func (db *GoLevelDB) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, errKeyEmpty
	}
	res, err := db.db.Get(key)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return res, nil
}

// Has implements DB.
func (db *GoLevelDB) Has(key []byte) (bool, error) {
	bytes, err := db.Get(key)
	if err != nil {
		return false, err
	}
	return bytes != nil, nil
}

// Set implements DB.
func (db *GoLevelDB) Set(key []byte, value []byte) error {
	if len(key) == 0 {
		return errKeyEmpty
	}
	if value == nil {
		return errValueNil
	}
	if err := db.db.Set(key, value); err != nil {
		return err
	}
	return nil
}

// SetSync implements DB.
func (db *GoLevelDB) SetSync(key []byte, value []byte) error {
	return db.db.Set(key, value)
}

// Delete implements DB.
func (db *GoLevelDB) Delete(key []byte) error {
	if len(key) == 0 {
		return errKeyEmpty
	}
	if err := db.db.Delete(key); err != nil {
		return err
	}
	return nil
}

// DeleteSync implements DB.
func (db *GoLevelDB) DeleteSync(key []byte) error {
	return db.db.Delete(key)
}

// Close implements DB.
func (db *GoLevelDB) Close() error {
	if err := db.db.Close(); err != nil {
		return err
	}
	return nil
}

// Print implements DB.
func (db *GoLevelDB) Print() error {
	return nil
}

// Stats implements DB.
func (db *GoLevelDB) Stats() map[string]string {
	return nil
}

// NewBatch implements DB.
func (db *GoLevelDB) NewBatch() tmdb.Batch {
	return newGoLevelDBBatch(db.db)
}

// Iterator implements DB.
func (db *GoLevelDB) Iterator(start, end []byte) (tmdb.Iterator, error) {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		return nil, errKeyEmpty
	}
	itr := db.db.DB().(*leveldb.DB).NewIterator(&util.Range{Start: start, Limit: end}, nil)
	return newGoLevelDBIterator(itr, start, end, false), nil
}

// ReverseIterator implements DB.
func (db *GoLevelDB) ReverseIterator(start, end []byte) (tmdb.Iterator, error) {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		return nil, errKeyEmpty
	}
	itr := db.db.DB().(*leveldb.DB).NewIterator(&util.Range{Start: start, Limit: end}, nil)
	return newGoLevelDBIterator(itr, start, end, true), nil
}