package leveldb

import (
	"encoding/binary"
	"errors"
	"github.com/syndtr/goleveldb/leveldb"
	"sync"

	"github.com/enigmampc/SecretNetwork/rumor-go/db"
)

var (
	ErrEmptyKey = errors.New("keyPrefix cannot be empty")
	ErrZeroBandwidth = errors.New("bandwidth must be > 0")
)

type Sequence struct {
	sync.Mutex
	db db.DB
	key []byte
	next uint64
	leased uint64
	bandwidth uint64
}

func NewLevelDBSequence(db db.DB, key []byte, bandwidth uint64) (db.Sequence, error) {
	switch {
	case len(key) == 0:
		return nil, ErrEmptyKey
	case bandwidth == 0:
		return nil, ErrZeroBandwidth
	}

	seq := &Sequence{
		db:        db,
		key:       key,
		next:      0,
		leased:    0,
		bandwidth: bandwidth,
	}

	err := seq.updateLease()
	return seq, err
}

func (seq *Sequence) Next() (uint64, error) {
	seq.Lock()
	defer seq.Unlock()
	if seq.next >= seq.leased {
		if err := seq.updateLease(); err != nil {
			return 0, err
		}
	}
	val := seq.next
	seq.next++
	return val, nil
}

func (seq *Sequence) Release() error {
	seq.Lock()
	defer seq.Unlock()

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], seq.next)
	// return txn.SetEntry(NewEntry(seq.key, buf[:]))

	if err := seq.db.Set(seq.key, buf[:]); err != nil {
		return err
	}

	seq.leased = seq.next
	return nil
}

func (seq *Sequence) updateLease() error {
	item, err := seq.db.Get(seq.key)
	switch {
	case err == leveldb.ErrNotFound:
		seq.next = 0
	case err != nil:
		return err
	default:
		var num = binary.BigEndian.Uint64(item)
		seq.next = num
	}

	lease := seq.next + seq.bandwidth
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], lease)
	if err := seq.db.Set(seq.key, buf[:]); err != nil {
		return err
	}
	seq.leased = lease
	return nil

}