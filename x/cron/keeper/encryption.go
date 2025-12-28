package keeper

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/miscreant/miscreant.go"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"

	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	regtypes "github.com/scrtlabs/SecretNetwork/x/registration"
)

var hkdfSalt = []byte{
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x02, 0x4b, 0xea, 0xd8, 0xdf, 0x69, 0x99,
	0x08, 0x52, 0xc2, 0x02, 0xdb, 0x0e, 0x00, 0x97,
	0xc1, 0xa1, 0x2e, 0xa6, 0x37, 0xd7, 0xe9, 0x6d,
}

// GetModulePrivateKey returns the artificially generated cron module's secp256k1 private key.
// The key is stored as a base64-encoded string and is used for signing scheduled transactions.
func GetModulePrivateKey() cryptotypes.PrivKey {
	privKeyBase64 := "fgAMxmXhxA/Gah7CtAM1/Li9Slmn5pHWc75XOUusPPQ="
	privKeyBytes, err := base64.StdEncoding.DecodeString(privKeyBase64)
	if err != nil {
		fmt.Printf("failed to decode private key: %v", err)
	}
	return &secp256k1.PrivKey{Key: privKeyBytes}
}

// GetModuleTxKeyPair returns a fixed Curve25519 keypair derived from the module's secp256k1 key.
// This keypair is used for transaction encryption/decryption and allows scheduled transactions
// to be decrypted using the standard secretcli query command.
func GetModuleTxKeyPair() ([]byte, []byte) {
	privKey := GetModulePrivateKey()
	privKeyBytes := privKey.Bytes()

	// Derive Curve25519 private key from secp256k1 key (hash it to get 32 bytes)
	txSenderPrivKey := sha256.Sum256(privKeyBytes)

	// Derive Curve25519 public key
	var txSenderPubKey [32]byte
	curve25519.ScalarBaseMult(&txSenderPubKey, &txSenderPrivKey)

	return txSenderPrivKey[:], txSenderPubKey[:]
}

// getTxEncryptionKey derives the transaction encryption key using the sender's private key,
// consensus IO public key, and nonce. This follows the same key derivation process used
// by regular transactions in Secret Network.
func getTxEncryptionKey(ctx sdk.Context, k *Keeper, txSenderPrivKey []byte, nonce []byte) ([]byte, error) {
	consensusIoPubKey := k.regKeeper.GetMasterKey(ctx, regtypes.MasterIoKeyId)

	txEncryptionIkm, err := curve25519.X25519(txSenderPrivKey, consensusIoPubKey.Bytes)
	if err != nil {
		ctx.Logger().Error("Failed to derive tx encryption key", "error", err)
		return nil, err
	}

	kdfFunc := hkdf.New(sha256.New, append(txEncryptionIkm, nonce...), hkdfSalt, []byte{})

	txEncryptionKey := make([]byte, 32)
	if _, err := io.ReadFull(kdfFunc, txEncryptionKey); err != nil {
		ctx.Logger().Error("Failed inside the getTxEncryptionKey", "error", err)
		return nil, err
	}

	return txEncryptionKey, nil
}

// Encrypt encrypts plaintext using a fixed Curve25519 keypair derived from the module's secp256k1 key.
// This allows scheduled transactions to be decrypted using the standard secretcli query command.
// The encryption uses AES-SIV (Synthetic Initialization Vector) mode for authenticated encryption.
func Encrypt(ctx sdk.Context, k *Keeper, plaintext []byte) ([]byte, error) {
	// Get fixed keypair derived from the module's secp256k1 key
	txSenderPrivKey, txSenderPubKey := GetModuleTxKeyPair()

	// Use random nonce (like regular transactions) instead of deterministic
	nonce := make([]byte, 32)
	_, err := rand.Read(nonce)
	if err != nil {
		ctx.Logger().Error("Failed to generate random nonce", "error", err)
		return nil, err
	}

	txEncryptionKey, err := getTxEncryptionKey(ctx, k, txSenderPrivKey, nonce)
	if err != nil {
		ctx.Logger().Error("Failed to get tx encryption key", "error", err)
		return nil, err
	}

	return encryptData(txEncryptionKey, txSenderPubKey, plaintext, nonce)
}

// encryptData performs the actual encryption using AES-SIV mode.
// The output format is: nonce(32 bytes) || wallet_pubkey(32 bytes) || ciphertext
func encryptData(aesEncryptionKey []byte, txSenderPubKey []byte, plaintext []byte, nonce []byte) ([]byte, error) {
	cipher, err := miscreant.NewAESCMACSIV(aesEncryptionKey)
	if err != nil {
		return nil, err
	}

	ciphertext, err := cipher.Seal(nil, plaintext, []byte{})
	if err != nil {
		return nil, err
	}

	// ciphertext = nonce(32) || wallet_pubkey(32) || ciphertext
	ciphertext = append(nonce, append(txSenderPubKey, ciphertext...)...)

	return ciphertext, nil
}
