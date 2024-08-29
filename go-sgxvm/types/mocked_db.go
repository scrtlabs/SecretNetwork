package types

import (
	"encoding/hex"
	"errors"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/hashicorp/go-memdb"
)

type Account struct {
	Address string // ethereum address
	Balance []byte // big-endian encoded Uint256 balance
	Nonce   uint64
	Code    []byte            // Contract code. Is nil if account is not a contract
	State   map[string][]byte // Contains state of the contract. Empty if account is not a contract
}

type MockedDB struct {
	db *memdb.MemDB
}

// CreateMockedDatabase creates an in-memory database that is used to keep changes between SGXVM execution on a Go side
func CreateMockedDatabase() MockedDB {
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"account": {
				Name: "account",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Address"},
					},
				},
			},
		},
	}

	db, err := memdb.NewMemDB(schema)
	if err != nil {
		panic(err) // We do not handle this error since this code is used only for testing
	}

	return MockedDB{db}
}

// GetAccount returns account struct stored in database
func (m MockedDB) GetAccount(address ethcommon.Address) (*Account, error) {
	txn := m.db.Txn(false)
	defer txn.Abort()

	raw, err := txn.First("account", "id", address.String())
	if err != nil {
		return &Account{}, err
	}

	if raw == nil {
		return nil, nil
	}

	return raw.(*Account), nil
}

// GetAccountOrEmpty returns found account or account with empty fields
func (m MockedDB) GetAccountOrEmpty(address ethcommon.Address) (Account, error) {
	acct, err := m.GetAccount(address)

	if err != nil {
		return Account{}, err
	}

	if acct == nil {
		return Account{
			Address: address.String(),
			Balance: make([]byte, 32),
			Nonce:   0,
			Code:    nil,
			State:   nil,
		}, nil
	}

	return Account{
		Address: acct.Address,
		Balance: acct.Balance,
		Nonce:   acct.Nonce,
		Code:    acct.Code,
		State:   acct.State,
	}, nil
}

// InsertAccount inserts new account with balance and nonce fields
func (m MockedDB) InsertAccount(address ethcommon.Address, balance []byte, nonce uint64) error {
	txn := m.db.Txn(true)

	acct, err := m.GetAccountOrEmpty(address)
	if err != nil {
		return err
	}

	updAcct := Account{
		Address: acct.Address,
		Balance: balance,
		Nonce:   nonce,
		Code:    acct.Code,
		State:   acct.State,
	}

	if err := txn.Insert("account", &updAcct); err != nil {
		return err
	}

	txn.Commit()
	return nil
}

// InsertContractCode inserts code of the contract
func (m MockedDB) InsertContractCode(address ethcommon.Address, code []byte) error {
	acct, err := m.GetAccount(address)
	if err != nil {
		return err
	}

	if acct == nil {
		return errors.New("cannot insert contract code. Account not found")
	}

	txn := m.db.Txn(true)
	updatedAcct := Account{
		Address: acct.Address,
		Balance: acct.Balance,
		Nonce:   acct.Nonce,
		Code:    code,
		State:   acct.State,
	}
	if err := txn.Insert("account", &updatedAcct); err != nil {
		return err
	}
	txn.Commit()
	return nil
}

// InsertStorageCell inserts new storage cell
func (m MockedDB) InsertStorageCell(address ethcommon.Address, key []byte, value []byte) error {
	acct, err := m.GetAccount(address)
	if err != nil {
		return err
	}

	if acct == nil {
		return errors.New("cannot insert storage cell. Account not found")
	}

	txn := m.db.Txn(true)

	hexKey := hex.EncodeToString(key)
	var stateMap = acct.State
	if stateMap == nil {
		stateMap = make(map[string][]byte)
	}
	stateMap[hexKey] = value

	updatedAcct := Account{
		Address: acct.Address,
		Balance: acct.Balance,
		Nonce:   acct.Nonce,
		Code:    acct.Code,
		State:   stateMap,
	}
	if err := txn.Insert("account", &updatedAcct); err != nil {
		return err
	}
	txn.Commit()
	return nil
}

// GetStorageCell returns value contained in the storage cell
func (m MockedDB) GetStorageCell(address ethcommon.Address, key []byte) ([]byte, error) {
	acct, err := m.GetAccount(address)
	if err != nil {
		return nil, err
	}

	if acct == nil {
		return nil, nil
	}

	hexKey := hex.EncodeToString(key)
	value, _ := acct.State[hexKey]

	return value, nil
}

// Contains checks if provided address presents in DB
func (m MockedDB) Contains(address ethcommon.Address) (bool, error) {
	acct, err := m.GetAccount(address)
	if err != nil {
		return false, err
	}

	return acct != nil, nil
}

// Delete removes account record from the database
func (m MockedDB) Delete(address ethcommon.Address) error {
	txn := m.db.Txn(true)
	return txn.Delete("account", Account{Address: address.String()})
}

func (m MockedDB) RemoveStorageCell(address ethcommon.Address, key []byte) error {
	txn := m.db.Txn(true)

	acct, err := m.GetAccount(address)
	if err != nil {
		return err
	}

	if acct == nil {
		return errors.New("cannot remove storage cell. Account not found")
	}

	hexKey := hex.EncodeToString(key)
	var stateMap = acct.State
	if stateMap == nil {
		stateMap = make(map[string][]byte)
	}
	delete(stateMap, hexKey)

	updatedAcct := Account{
		Address: acct.Address,
		Balance: acct.Balance,
		Nonce:   acct.Nonce,
		Code:    acct.Code,
		State:   stateMap,
	}
	if err := txn.Insert("account", &updatedAcct); err != nil {
		return err
	}
	txn.Commit()
	return nil
}
