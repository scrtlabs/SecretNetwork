package leveldb

import (
	"bytes"
	"github.com/syndtr/goleveldb/leveldb"
	leveldbIterator "github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	tmdb "github.com/tendermint/tm-db"

	"github.com/enigmampc/SecretNetwork/rumor-go/db"
	tmadapter "github.com/enigmampc/SecretNetwork/rumor-go/db/leveldb/tm_adapter"
	"github.com/enigmampc/SecretNetwork/rumor-go/utils"
)

type LevelDB struct {
	db   *leveldb.DB
	path string
	gt   db.Batch
}

// type check
var _ db.DB = (*LevelDB)(nil)

func NewLevelDB(path string) *LevelDB {
	dbInstance := &LevelDB{
		path: path,
		gt:   nil,
	}

	dbInstance.db = dbInstance.open(path)

	return dbInstance
}

func (ldb *LevelDB) open(path string) *leveldb.DB {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		panic(err)
	}

	return db
}

func (ldb *LevelDB) DB() interface{} {
	return ldb.db
}

func (ldb *LevelDB) SetCriticalZone() {
	ldb.gt = ldb.Batch()
}

func (ldb *LevelDB) ReleaseCriticalZone() error {
	defer func() {
		ldb.gt = nil
	}()

	if ldb.gt == nil {
		return nil
	}

	return ldb.gt.FlushGT()
}

// Purge safely closes db, ignoring any uncommitted transactions
func (ldb *LevelDB) Purge(purgeTransaction bool) {
	if purgeTransaction {
		ldb.gt.Purge()
		ldb.ReleaseCriticalZone()
	}
	ldb.Close()
}

func (ldb *LevelDB) GetCosmosAdapter() tmdb.DB {
	return tmadapter.NewCosmosAdapter(ldb)
}

func (ldb *LevelDB) GetDB() *leveldb.DB {
	return ldb.db
}

func (ldb *LevelDB) Get(key []byte) ([]byte, error) {
	if ldb.gt != nil {
		return ldb.db.Get(key, nil)
	} else {
		return ldb.db.Get(key, nil)
	}
}

func (ldb *LevelDB) Set(key, data []byte) error {
	if ldb.gt != nil {
		return ldb.gt.Set(key, data)
	} else {
		return ldb.db.Put(key, data, nil)
	}
}

func (ldb *LevelDB) Delete(key []byte) error {
	if ldb.gt != nil {
		return ldb.gt.Delete(key)
	} else {
		return ldb.db.Delete(key, nil)
	}
}

func (ldb *LevelDB) GetSequence(key []byte, bandwidth uint64) (db.Sequence, error) {
	return NewLevelDBSequence(ldb, key, bandwidth)
}

func (ldb *LevelDB) Close() error {
	return ldb.db.Close()
}

type LevelIterator struct {
	it             leveldbIterator.Iterator
	start          []byte
	indexKeyLength int
	reverse        bool
}

func (ldb *LevelDB) Iterator(
	start []byte,
	reverse bool,
) db.Iterator {
	var it leveldbIterator.Iterator

	// if iterator goes backwards, so start needs to be the biggest of that index start range
	if reverse {
		limit := utils.GetReverseSeekKeyFromIndexGroupPrefix(start)
		it = ldb.db.NewIterator(&util.Range{Start: nil, Limit: limit}, nil)
		it.Last()
	} else {
		it = ldb.db.NewIterator(&util.Range{Start: start, Limit: nil}, nil)
		it.First()
	}

	return &LevelIterator{
		it:             it,
		start:          start,
		indexKeyLength: len(start),
		reverse:        reverse,
	}
}

// no index-only iterator for ldb
func (ldb *LevelDB) IndexIterator(
	start []byte,
	reverse bool,
) db.Iterator {
	return ldb.Iterator(start, reverse)
}

func (it *LevelIterator) Close() {
	it.it.Release()
}

func (it *LevelIterator) Valid(prefix []byte) bool {
	if len(prefix) != 0 && !bytes.HasPrefix(it.it.Key(), prefix) {
		return false
	}
	return it.it.Valid()
}

func (it *LevelIterator) Next() {
	if it.reverse {
		it.it.Prev()
	} else {
		it.it.Next()
	}
}

func (it *LevelIterator) Key() []byte {
	return it.it.Key()
}

func (it *LevelIterator) Value() []byte {
	return it.it.Value()
}

func (it *LevelIterator) DocumentKey() []byte {
	return it.Key()[it.indexKeyLength:]
}

type Batch struct {
	batch *leveldb.Batch
	db    *leveldb.DB
}

func (ldb *LevelDB) Batch() db.Batch {
	if ldb.gt != nil {
		return ldb.gt
	} else {
		return &Batch{
			batch: new(leveldb.Batch),
			db:    ldb.db,
		}
	}
}

func (batch *Batch) Set(key, data []byte) error {
	batch.batch.Put(key, data)
	return nil
}

func (batch *Batch) Delete(key []byte) error {
	batch.batch.Delete(key)
	return nil
}

func (batch *Batch) Flush() error {
	// noop; all Flush() calls should be later called with FlushGT
	return nil
}

func (batch *Batch) Purge() {
	if batch.batch == nil {
		return
	}
	batch.batch.Reset()
}

func (batch *Batch) FlushGT() error {
	defer batch.batch.Reset()
	return batch.db.Write(batch.batch, &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         true,
	})
}

func (batch *Batch) Close() {
	// noop, batch should never be closed on its own
}
