package tm_adapter

import (
	tmdb "github.com/tendermint/tm-db"

	"github.com/enigmampc/SecretNetwork/rumor-go/db"
)

type goLevelDBBatch struct {
	db db.DB
	batch db.Batch
}

var _ tmdb.Batch = (*goLevelDBBatch)(nil)

func newGoLevelDBBatch(ldb db.DB) *goLevelDBBatch {
	return &goLevelDBBatch{
		db:    ldb,
		batch: ldb.Batch(),
	}
}

func (b *goLevelDBBatch) assertOpen() {
	if b.batch == nil {
		panic("batch has been written or closed")
	}
}

// Set implements Batch.
func (b *goLevelDBBatch) Set(key, value []byte) {
	b.assertOpen()
	b.batch.Set(key, value)
}

// Delete implements Batch.
func (b *goLevelDBBatch) Delete(key []byte) {
	b.assertOpen()
	b.batch.Delete(key)
}

// Write implements Batch.
func (b *goLevelDBBatch) Write() error {
	return b.write(false)
}

// WriteSync implements Batch.
func (b *goLevelDBBatch) WriteSync() error {
	return b.write(true)
}

func (b *goLevelDBBatch) write(sync bool) error {
	// noop; should be handled by driver global transaction
	return nil
}

// Close implements Batch.
func (b *goLevelDBBatch) Close() {
	// noop; should be handled by driver global transaction
}