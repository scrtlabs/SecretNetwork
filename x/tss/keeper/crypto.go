package keeper

import (
	"crypto/rand"
	"crypto/sha512"
	"fmt"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
)

// Ed25519ToX25519PublicKey converts an Ed25519 public key to an X25519 public key
// This follows the standard conversion used by libsodium and age encryption
func Ed25519ToX25519PublicKey(ed25519PubKey []byte) ([]byte, error) {
	if len(ed25519PubKey) != 32 {
		return nil, fmt.Errorf("invalid Ed25519 public key length: %d", len(ed25519PubKey))
	}

	// Parse the Ed25519 public key as a point on the Edwards curve
	edPoint, err := new(edwards25519.Point).SetBytes(ed25519PubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Ed25519 public key: %w", err)
	}

	// Convert to Montgomery form (X25519)
	return edPoint.BytesMontgomery(), nil
}

// Ed25519ToX25519PrivateKey converts an Ed25519 private key (seed) to an X25519 private key
// The Ed25519 private key should be the 32-byte seed (not the 64-byte expanded key)
func Ed25519ToX25519PrivateKey(ed25519PrivKey []byte) ([]byte, error) {
	if len(ed25519PrivKey) != 32 && len(ed25519PrivKey) != 64 {
		return nil, fmt.Errorf("invalid Ed25519 private key length: %d", len(ed25519PrivKey))
	}

	// If 64-byte key provided, use first 32 bytes (the seed)
	seed := ed25519PrivKey
	if len(ed25519PrivKey) == 64 {
		seed = ed25519PrivKey[:32]
	}

	// Hash the seed with SHA-512 (same as Ed25519 key derivation)
	h := sha512.Sum512(seed)

	// Clamp the first 32 bytes to get the X25519 private key
	// This matches the clamping done in Ed25519 scalar multiplication
	h[0] &= 248
	h[31] &= 127
	h[31] |= 64

	return h[:32], nil
}

// EncryptedKeyShare contains the encrypted data and metadata needed for decryption
type EncryptedKeyShare struct {
	// Ciphertext is the encrypted secret share (NaCl box format)
	Ciphertext []byte
	// EphemeralPubKey is the sender's ephemeral public key (needed for decryption)
	EphemeralPubKey []byte
	// Nonce used for encryption (24 bytes, included in ciphertext for simplicity)
	Nonce []byte
}

// EncryptForValidator encrypts data for a specific validator using their Ed25519 public key
// Uses ephemeral key pair for forward secrecy (similar to ECIES)
func EncryptForValidator(plaintext []byte, validatorEd25519PubKey []byte) (*EncryptedKeyShare, error) {
	// Convert validator's Ed25519 pubkey to X25519
	recipientPubKey, err := Ed25519ToX25519PublicKey(validatorEd25519PubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert validator pubkey: %w", err)
	}

	// Generate ephemeral key pair for this encryption
	ephemeralPubKey, ephemeralPrivKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	// Generate random nonce
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Convert recipient pubkey to array
	var recipientPubKeyArr [32]byte
	copy(recipientPubKeyArr[:], recipientPubKey)

	// Encrypt using NaCl box (authenticated encryption)
	ciphertext := box.Seal(nil, plaintext, &nonce, &recipientPubKeyArr, ephemeralPrivKey)

	return &EncryptedKeyShare{
		Ciphertext:      ciphertext,
		EphemeralPubKey: ephemeralPubKey[:],
		Nonce:           nonce[:],
	}, nil
}

// DecryptKeyShare decrypts an encrypted key share using the validator's Ed25519 private key
func DecryptKeyShare(encrypted *EncryptedKeyShare, validatorEd25519PrivKey []byte) ([]byte, error) {
	// Convert validator's Ed25519 private key to X25519
	recipientPrivKey, err := Ed25519ToX25519PrivateKey(validatorEd25519PrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert validator privkey: %w", err)
	}

	// Convert to arrays
	var recipientPrivKeyArr [32]byte
	copy(recipientPrivKeyArr[:], recipientPrivKey)

	var senderPubKeyArr [32]byte
	copy(senderPubKeyArr[:], encrypted.EphemeralPubKey)

	var nonce [24]byte
	copy(nonce[:], encrypted.Nonce)

	// Decrypt using NaCl box
	plaintext, ok := box.Open(nil, encrypted.Ciphertext, &nonce, &senderPubKeyArr, &recipientPrivKeyArr)
	if !ok {
		return nil, fmt.Errorf("decryption failed: authentication error")
	}

	return plaintext, nil
}

// EncryptKeyShareForChain encrypts a key share and returns the components for on-chain storage
// Returns: encryptedData (ciphertext with prepended nonce), ephemeralPubKey
func EncryptKeyShareForChain(plaintext []byte, validatorEd25519PubKey []byte) (encryptedData []byte, ephemeralPubKey []byte, err error) {
	encrypted, err := EncryptForValidator(plaintext, validatorEd25519PubKey)
	if err != nil {
		return nil, nil, err
	}

	// Prepend nonce to ciphertext for storage (nonce || ciphertext)
	encryptedData = make([]byte, len(encrypted.Nonce)+len(encrypted.Ciphertext))
	copy(encryptedData[:24], encrypted.Nonce)
	copy(encryptedData[24:], encrypted.Ciphertext)

	return encryptedData, encrypted.EphemeralPubKey, nil
}

// DecryptKeyShareFromChain decrypts a key share stored on-chain
func DecryptKeyShareFromChain(encryptedData []byte, ephemeralPubKey []byte, validatorEd25519PrivKey []byte) ([]byte, error) {
	if len(encryptedData) < 24 {
		return nil, fmt.Errorf("encrypted data too short")
	}

	encrypted := &EncryptedKeyShare{
		Nonce:           encryptedData[:24],
		Ciphertext:      encryptedData[24:],
		EphemeralPubKey: ephemeralPubKey,
	}

	return DecryptKeyShare(encrypted, validatorEd25519PrivKey)
}

// DeriveSharedSecret derives a shared secret from X25519 key exchange
// Used internally by NaCl box, but exposed here for testing
func DeriveSharedSecret(privateKey, publicKey []byte) ([]byte, error) {
	if len(privateKey) != 32 || len(publicKey) != 32 {
		return nil, fmt.Errorf("invalid key length")
	}

	shared, err := curve25519.X25519(privateKey, publicKey)
	if err != nil {
		return nil, fmt.Errorf("key exchange failed: %w", err)
	}

	return shared, nil
}
