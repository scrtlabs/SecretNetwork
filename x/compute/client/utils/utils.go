package utils

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/cosmos/cosmos-sdk/client/context"
	regtypes "github.com/enigmampc/EnigmaBlockchain/x/registration"
	ra "github.com/enigmampc/EnigmaBlockchain/x/registration/remote_attestation"
	"github.com/miscreant/miscreant.go"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
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

// WASMContext wraps github.com/cosmos/cosmos-sdk/client/context.CLIContext
type WASMContext struct {
	CLIContext       context.CLIContext
	TestKeyPairPath  string
	TestMasterIOCert regtypes.MasterCertificate
}

type keyPair struct {
	Private string `json:"private"`
	Public  string `json:"public"`
}

// GetTxSenderKeyPair get the local tx encryption id
func (ctx WASMContext) GetTxSenderKeyPair() (privkey []byte, pubkey []byte, er error) {
	var keyPairFilePath string
	if len(ctx.TestKeyPairPath) > 0 {
		keyPairFilePath = ctx.TestKeyPairPath
	} else {
		keyPairFilePath = path.Join(ctx.CLIContext.HomeDir, "id_tx_io.json")
	}

	if _, err := os.Stat(keyPairFilePath); os.IsNotExist(err) {
		var privkey [32]byte
		rand.Read(privkey[:])

		var pubkey [32]byte
		curve25519.ScalarBaseMult(&pubkey, &privkey)

		keyPair := keyPair{
			Private: hex.EncodeToString(privkey[:]),
			Public:  hex.EncodeToString(pubkey[:]),
		}

		keyPairJSONBytes, err := json.MarshalIndent(keyPair, "", "    ")
		if err != nil {
			return nil, nil, err
		}

		err = ioutil.WriteFile(keyPairFilePath, keyPairJSONBytes, 0644)
		if err != nil {
			return nil, nil, err
		}

		return privkey[:], pubkey[:], nil
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

	privkey, err = hex.DecodeString(keyPair.Private)
	pubkey, err = hex.DecodeString(keyPair.Public)

	// TODO verify pubkey

	return privkey, pubkey, nil
}

var hkdfSalt = []byte{
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x02, 0x4b, 0xea, 0xd8, 0xdf, 0x69, 0x99,
	0x08, 0x52, 0xc2, 0x02, 0xdb, 0x0e, 0x00, 0x97,
	0xc1, 0xa1, 0x2e, 0xa6, 0x37, 0xd7, 0xe9, 0x6d,
}

func (ctx WASMContext) getConsensusIoPubKey() ([]byte, error) {
	var certs regtypes.GenesisState
	if ctx.TestMasterIOCert != nil { // TODO check length?
		certs.IoMasterCertificate = ctx.TestMasterIOCert
	} else {
		res, _, err := ctx.CLIContext.Query("custom/register/master-cert")
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(res, &certs)
		if err != nil {
			return nil, err
		}
	}

	ioPubkey, err := ra.VerifyRaCert(certs.IoMasterCertificate)
	if err != nil {
		return nil, err
	}

	return ioPubkey, nil
}

func (ctx WASMContext) getTxEncryptionKey(txSenderPrivKey []byte, nonce []byte) ([]byte, error) {
	consensusIoPubKeyBytes, err := ctx.getConsensusIoPubKey()
	if err != nil {
		return nil, err
	}

	txEncryptionIkm, err := curve25519.X25519(txSenderPrivKey, consensusIoPubKeyBytes)

	kdfFunc := hkdf.New(sha256.New, append(txEncryptionIkm[:], nonce...), hkdfSalt, []byte{})

	txEncryptionKey := make([]byte, 32)
	if _, err := io.ReadFull(kdfFunc, txEncryptionKey); err != nil {
		return nil, err
	}

	return txEncryptionKey, nil
}

// Encrypt encrypts
func (ctx WASMContext) Encrypt(plaintext []byte) ([]byte, error) {
	txSenderPrivKey, txSenderPubKey, err := ctx.GetTxSenderKeyPair()

	nonce := make([]byte, 32)
	_, _ = rand.Read(nonce)

	txEncryptionKey, err := ctx.getTxEncryptionKey(txSenderPrivKey, nonce)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	cipher, err := miscreant.NewAESCMACSIV(txEncryptionKey)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	ciphertext, err := cipher.Seal(nil, plaintext, []byte{})
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// ciphertext = nonce(32) || wallet_pubkey(32) || ciphertext
	ciphertext = append(nonce, append(txSenderPubKey, ciphertext...)...)

	return ciphertext, nil
}

// Decrypt decrypts
func (ctx WASMContext) Decrypt(ciphertext []byte, nonce []byte) ([]byte, error) {
	txSenderPrivKey, _, err := ctx.GetTxSenderKeyPair()

	txEncryptionKey, err := ctx.getTxEncryptionKey(txSenderPrivKey, nonce)
	if err != nil {
		return nil, err
	}

	cipher, err := miscreant.NewAESCMACSIV(txEncryptionKey)
	if err != nil {
		return nil, err
	}

	return cipher.Open(nil, ciphertext, []byte{})
}
