package cli

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	flag "github.com/spf13/pflag"
)

type argumentDecoder struct {
	dec                func(string) ([]byte, error)
	asciiF, hexF, b64F bool
}

func newArgDecoder(def func(string) ([]byte, error)) *argumentDecoder {
	return &argumentDecoder{dec: def}
}

func (a *argumentDecoder) RegisterFlags(f *flag.FlagSet, argName string) {
	f.BoolVar(&a.asciiF, "ascii", false, "ascii encoded "+argName)
	f.BoolVar(&a.hexF, "hex", false, "hex encoded "+argName)
	f.BoolVar(&a.b64F, "b64", false, "base64 encoded "+argName)
}

func (a *argumentDecoder) DecodeString(s string) ([]byte, error) {
	found := -1
	for i, v := range []*bool{&a.asciiF, &a.hexF, &a.b64F} {
		if !*v {
			continue
		}
		if found != -1 {
			return nil, errors.New("multiple decoding flags used")
		}
		found = i
	}
	switch found {
	case 0:
		return asciiDecodeString(s)
	case 1:
		return hex.DecodeString(s)
	case 2:
		return base64.StdEncoding.DecodeString(s)
	default:
		return a.dec(s)
	}
}

func asciiDecodeString(s string) ([]byte, error) {
	return []byte(s), nil
}

func parseEncryptedBlob(blob []byte) ([]byte, []byte, []byte, error) {
	if len(blob) < 64 {
		return nil, nil, nil, fmt.Errorf("input must be > 64 bytes. Got %d", len(blob))
	}

	nonce := blob[0:32]
	originalTxSenderPubkey := blob[32:64]
	ciphertextInput := blob[64:]

	return nonce, originalTxSenderPubkey, ciphertextInput, nil
}
