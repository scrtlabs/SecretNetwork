package utils

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"io"

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

// Encrypt encrypts the input ([]byte)
// https://gist.github.com/kkirsche/e28da6754c39d5e7ea10
func Encrypt(plaintext []byte) ([]byte, error) {
	// The key argument should be the AES key, either 16 or 32 bytes
	// to select AES-128 or AES-256.
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

	// aad = nonce(32)|wallet_pubkey(33) = 65 bytes
	ad := []byte{}
	ad = append(ad, nonce...)        // TODO fix real inputNonce
	ad = append(ad, walletPubKey...) // TODO fix real outputNonce

	ciphertext = append(ad, ciphertext...)

	return ciphertext, nil
}

// Decrypt decrypts the input ([]byte)
// https://gist.github.com/kkirsche/e28da6754c39d5e7ea10
func Decrypt(ciphertext []byte) ([]byte, error) {
	// The key argument should be the AES key, either 16 or 32 bytes
	// to select AES-128 or AES-256.
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
