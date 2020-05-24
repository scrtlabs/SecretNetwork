package utils

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
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

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	inputNonce := make([]byte, 12) // 96 bit nonce
	if _, err := io.ReadFull(rand.Reader, inputNonce); err != nil {
		return nil, err
	}

	outputNonce := make([]byte, 12) // 96 bit nonce
	if _, err := io.ReadFull(rand.Reader, outputNonce); err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	inputNonce = make([]byte, 12)  // TODO fix
	outputNonce = make([]byte, 12) // TODO fix

	// aad = inputNonce(12)|wallet_pubkey(65)|outputNonce(12) = 89 bytes
	aad := []byte{}
	aad = append(aad, inputNonce...)       // TODO fix real inputNonce
	aad = append(aad, make([]byte, 65)...) // TODO fix real wallet_pubkey
	aad = append(aad, outputNonce...)      // TODO fix real outputNonce
	aad = []byte{}

	ciphertext := aesgcm.Seal(nil, inputNonce, plaintext, aad)
	ciphertext = append(ciphertext, aad...)

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

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// extract nonce
	// nonce is appended at the end
	// nonce is 96 bits / 12 bytes
	// outputNonce := ciphertext[len(ciphertext)-12:]
	// ciphertext = ciphertext[0 : len(ciphertext)-12]
	// aad := outputNonce

	outputNonce := make([]byte, 12) // TODO fix
	aad := []byte{}                 // TODO fix

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return aesgcm.Open(nil, outputNonce, ciphertext, aad)
}
