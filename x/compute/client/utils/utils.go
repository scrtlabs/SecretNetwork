package utils

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/cosmos/cosmos-sdk/client"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"

	regtypes "github.com/enigmampc/SecretNetwork/x/registration"
	ra "github.com/enigmampc/SecretNetwork/x/registration/remote_attestation"

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

// WASMContext wraps github.com/cosmos/cosmos-sdk/client/client.Context
type WASMContext struct {
	CLIContext       client.Context
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
		keyPairFilePath = filepath.Join(ctx.CLIContext.HomeDir, "id_tx_io.json")
	}

	if _, err := os.Stat(keyPairFilePath); os.IsNotExist(err) {
		var privkey [32]byte
		rand.Read(privkey[:]) //nolint:errcheck

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

		err = os.WriteFile(keyPairFilePath, keyPairJSONBytes, 0o600)
		if err != nil {
			return nil, nil, err
		}

		return privkey[:], pubkey[:], nil
	}

	keyPairJSONBytes, err := os.ReadFile(keyPairFilePath)
	if err != nil {
		return nil, nil, err
	}

	var keyPair keyPair

	err = json.Unmarshal(keyPairJSONBytes, &keyPair)
	if err != nil {
		return nil, nil, err
	}

	privkey, err = hex.DecodeString(keyPair.Private)
	if err != nil {
		return nil, nil, err
	}
	pubkey, err = hex.DecodeString(keyPair.Public)
	if err != nil {
		return nil, nil, err
	}

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
	var masterIoKey regtypes.Key
	if ctx.TestMasterIOCert.Bytes != nil { // TODO check length?
		masterIoKey.Key = ctx.TestMasterIOCert.Bytes
	} else {
		res, _, err := ctx.CLIContext.Query("/secret.registration.v1beta1.Query/TxKey")
		if err != nil {
			return nil, err
		}

		err = encoding.GetCodec(proto.Name).Unmarshal(res, &masterIoKey)
		if err != nil {
			return nil, err
		}
	}

	ioPubkey, err := ra.VerifyRaCert(masterIoKey.Key)
	if err != nil {
		return nil, err
	}

	return ioPubkey, nil
}

func (ctx WASMContext) getTxEncryptionKey(txSenderPrivKey []byte, nonce []byte) ([]byte, error) {
	consensusIoPubKeyBytes, err := ctx.getConsensusIoPubKey()
	if err != nil {
		fmt.Println("Failed to get IO key. Make sure the CLI and the node you are targeting are operating in the same SGX mode")
		return nil, err
	}

	txEncryptionIkm, err := curve25519.X25519(txSenderPrivKey, consensusIoPubKeyBytes)
	if err != nil {
		fmt.Println("Failed to get tx encryption key")
		return nil, err
	}

	kdfFunc := hkdf.New(sha256.New, append(txEncryptionIkm, nonce...), hkdfSalt, []byte{})

	txEncryptionKey := make([]byte, 32)
	if _, err := io.ReadFull(kdfFunc, txEncryptionKey); err != nil {
		return nil, err
	}

	return txEncryptionKey, nil
}

func (ctx WASMContext) OfflineEncrypt(plaintext []byte, pathToMasterIoKey string) ([]byte, error) {
	// parse coins trying to be sent
	cert, err := os.ReadFile(pathToMasterIoKey)
	if err != nil {
		return nil, err
	}

	pubkey, err := ra.VerifyRaCert(cert)
	if err != nil {
		return nil, err
	}

	txSenderPrivKey, txSenderPubKey, err := ctx.GetTxSenderKeyPair()
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, 32)
	_, err = rand.Read(nonce)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	txEncryptionKey, err := GetTxEncryptionKeyOffline(pubkey, txSenderPrivKey, nonce)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return encryptData(txEncryptionKey, txSenderPubKey, plaintext, nonce)
}

// Encrypt encrypts
func (ctx WASMContext) Encrypt(plaintext []byte) ([]byte, error) {
	txSenderPrivKey, txSenderPubKey, err := ctx.GetTxSenderKeyPair()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	nonce := make([]byte, 32)
	_, err = rand.Read(nonce)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	txEncryptionKey, err := ctx.getTxEncryptionKey(txSenderPrivKey, nonce)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return encryptData(txEncryptionKey, txSenderPubKey, plaintext, nonce)
}

// Decrypt decrypts
func (ctx WASMContext) Decrypt(ciphertext []byte, nonce []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return []byte{}, nil
	}

	txSenderPrivKey, _, err := ctx.GetTxSenderKeyPair()
	if err != nil {
		return nil, err
	}

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

var re = regexp.MustCompile("encrypted: (.+?):")

func (ctx WASMContext) DecryptError(errString string, msgType string, nonce []byte) (json.RawMessage, error) {
	regexMatch := re.FindStringSubmatch(errString)
	if len(regexMatch) != 2 {
		return nil, fmt.Errorf("got an error finding base64 of the error: regexMatch '%v' should have a length of 2. error: %v", regexMatch, errString)
	}
	errorCipherB64 := regexMatch[1]

	errorCipherBz, err := base64.StdEncoding.DecodeString(errorCipherB64)
	if err != nil {
		return nil, fmt.Errorf("got an error decoding base64 of the error: %w", err)
	}

	errorPlainBz, err := ctx.Decrypt(errorCipherBz, nonce)
	if err != nil {
		return nil, fmt.Errorf("got an error decrypting the error: %w", err)
	}

	return errorPlainBz, nil
}

func encryptData(aesEncryptionKey []byte, txSenderPubKey []byte, plaintext []byte, nonce []byte) ([]byte, error) {
	cipher, err := miscreant.NewAESCMACSIV(aesEncryptionKey)
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
	ciphertext = append(nonce, append(txSenderPubKey, ciphertext...)...) //nolint:gocritic

	return ciphertext, nil
}

func GetTxEncryptionKeyOffline(pubkey []byte, txSenderPrivKey []byte, nonce []byte) ([]byte, error) {
	txEncryptionIkm, err := curve25519.X25519(txSenderPrivKey, pubkey)
	if err != nil {
		return nil, err
	}

	kdfFunc := hkdf.New(sha256.New, append(txEncryptionIkm, nonce...), hkdfSalt, []byte{})

	txEncryptionKey := make([]byte, 32)
	if _, err := io.ReadFull(kdfFunc, txEncryptionKey); err != nil {
		return nil, err
	}

	return txEncryptionKey, nil
}
