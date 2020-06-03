package utils

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/miscreant/miscreant.go"
)

var (
	gzipIdent = []byte("\x1F\x8B\x08")
	wasmIdent = []byte("\x00\x61\x73\x6D")
)

// IsGzip returns checks if the file contents are gzip compressed
func IsGzip(input []byte) bool {
	return bytes.Equal(input[:3], gzipIdent)
}

// IsWasm checks if the file contents are of wasm binary
func IsWasm(input []byte) bool {
	return bytes.Equal(input[:4], wasmIdent)
}

// GzipIt compresses the input ([]byte)
func GzipIt(input []byte) ([]byte, error) {
	// Create gzip writer.
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, err := w.Write(input)
	if err != nil {
		return nil, err
	}
	err = w.Close() // You must close this first to flush the bytes to the buffer.
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// WASMCLIContext wraps github.com/cosmos/cosmos-sdk/client/context.CLIContext
type WASMCLIContext struct {
	CLIContext context.CLIContext
}

type keyPair struct {
	Private string `json:"private"`
	Public  string `json:"public"`
}

func (ctx WASMCLIContext) getKeyPair() (*secp256k1.PrivateKey, *secp256k1.PublicKey, error) {
	keyPairFilePath := path.Join(ctx.CLIContext.HomeDir, "id_tx_io.json")

	if _, err := os.Stat(keyPairFilePath); os.IsNotExist(err) {
		privkey, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			return nil, nil, err
		}

		pubkey := privkey.PubKey()

		keyPair := keyPair{
			Private: hex.EncodeToString(privkey.Serialize()),
			Public:  hex.EncodeToString(pubkey.Serialize()),
		}

		keyPairJSONBytes, err := json.MarshalIndent(keyPair, "", "    ")
		if err != nil {
			return nil, nil, err
		}

		err = ioutil.WriteFile(keyPairFilePath, keyPairJSONBytes, 0644)
		if err != nil {
			return nil, nil, err
		}

		return privkey, pubkey, nil
	}

	keyPairJSONBytes, err := ioutil.ReadFile(keyPairFilePath)
	if err != nil {
		return nil, nil, err
	}

	var keyPair keyPair

	err = json.Unmarshal(keyPairJSONBytes, &keyPair)
	if err != nil {
		return nil, nil, err
	}

	privKeyBytes, err := hex.DecodeString(keyPair.Private)

	privkey, pubkey := secp256k1.PrivKeyFromBytes(privKeyBytes)
	return privkey, pubkey, nil
}

// Encrypt encrypts the input ([]byte)
// https://gist.github.com/kkirsche/e28da6754c39d5e7ea10
func (ctx WASMCLIContext) Encrypt(plaintext []byte) ([]byte, error) {
	priv, pub, err := ctx.getKeyPair()
	log.Printf("priv: %v, pub %v, err %v", priv, pub, err)

	key := []byte{
		0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07,
		0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07,
		0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07,
		0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07,
	}

	cipher, err := miscreant.NewAESCMACSIV(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext, err := cipher.Seal(nil, plaintext, []byte{})
	if err != nil {
		return nil, err
	}

	nonce = make([]byte, 32)         // TODO fix
	walletPubKey := make([]byte, 33) // TODO fix

	// ad = nonce(32)|wallet_pubkey(33) = 65 bytes
	ad := []byte{}
	ad = append(ad, nonce...)        // TODO fix real inputNonce
	ad = append(ad, walletPubKey...) // TODO fix real outputNonce

	ciphertext = append(ad, ciphertext...)

	return ciphertext, nil
}

// Decrypt decrypts the input ([]byte)
// https://gist.github.com/kkirsche/e28da6754c39d5e7ea10
func (ctx WASMCLIContext) Decrypt(ciphertext []byte) ([]byte, error) {

	key := []byte{
		0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07,
		0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07,
		0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07,
		0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07,
	}

	cipher, err := miscreant.NewAESCMACSIV(key)
	if err != nil {
		return nil, err
	}

	// extract nonce
	// nonce is appended at the end
	// nonce is 96 bits / 12 bytes
	// outputNonce := ciphertext[len(ciphertext)-12:]
	// ciphertext = ciphertext[0 : len(ciphertext)-12]
	// aad := outputNonce

	// outputNonce := make([]byte, 32) // TODO fix
	// ad := []byte{} // TODO fix

	return cipher.Open(nil, ciphertext, []byte{})
}
