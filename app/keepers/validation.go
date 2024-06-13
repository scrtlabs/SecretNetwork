package keepers

import "errors"

const MaxMaxMemoCharacters = 1 << 10

func valMaxMemoCharacters(value interface{}) error {
	i, ok := value.(uint64)
	if !ok {
		return errors.New("invalid value type")
	}
	if i > MaxMaxMemoCharacters {
		return errors.New("exceeds max allowed value")
	}
	return nil
}

const MaxTxSigLimit uint64 = 1 << 4

func valTxSigLimit(value interface{}) error {
	i, ok := value.(uint64)
	if !ok {
		return errors.New("invalid value type")
	}
	if i > MaxTxSigLimit {
		return errors.New("exceeds max allowed value")
	}
	return nil
}

const MaxTxSizeCostPerByte uint64 = 1 << 40

func valTxSizeCostPerByte(value interface{}) error {
	i, ok := value.(uint64)
	if !ok {
		return errors.New("invalid value type")
	}
	if i > MaxTxSizeCostPerByte {
		return errors.New("exceeds max allowed value")
	}
	return nil
}

const MaxSigVerifyCostED25519 uint64 = 1 << 40

func valSigVerifyCostED25519(value interface{}) error {
	i, ok := value.(uint64)
	if !ok {
		return errors.New("invalid value type")
	}
	if i > MaxSigVerifyCostED25519 {
		return errors.New("exceeds max allowed value")
	}
	return nil
}

const MaxSigVerifyCostSecp256k1 uint64 = 1 << 40

func valSigVerifyCostSecp256k1(value interface{}) error {
	i, ok := value.(uint64)
	if !ok {
		return errors.New("invalid value type")
	}
	if i > 1024 {
		return errors.New("exceeds max allowed value")
	}
	return nil
}
