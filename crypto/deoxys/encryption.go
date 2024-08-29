package deoxys

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/oasisprotocol/deoxysii"
	"golang.org/x/crypto/curve25519"
)

// EncryptState encrypts smart contract state using simmetric key derived from master key only for specific contract.
// That allows us to improve cryptographic strength of our encryption scheme.
//
// As an output, this function returns vector which contains 15 bytes nonce and ciphertext.
func EncryptState(masterKey, contractAddress, value []byte) ([]byte, error) {
	// Derive tx encryption key
	txKey := DeriveEncryptionKey(masterKey, []byte("StateEncryptionKeyV1"))
	// Derive contract encryption key
	contractKey := DeriveEncryptionKey(txKey, contractAddress)
	// Encrypt storage value
	return encryptDeoxys(contractKey, value)
}

func DecryptState(masterKey, contractAddress, value []byte) ([]byte, error) {
	// Derive tx encryption key
	txKey := DeriveEncryptionKey(masterKey, []byte("StateEncryptionKeyV1"))
	// Derive contract encryption key
	contractKey := DeriveEncryptionKey(txKey, contractAddress)
	// Encrypt storage value
	return decryptDeoxys(contractKey, value)
}

// EncryptECDH encrypts provided value using encryption key, derived from user private key and node public key.
func EncryptECDH(privateKey, nodePublicKey, data []byte) ([]byte, error) {
	// Check length of node public key
	if len(nodePublicKey) != 32 {
		return nil, fmt.Errorf("wrong public key size. Expected 32, got %d", len(nodePublicKey))
	}
	// Derive shared secret
	sharedSecret, err := diffieHellman(privateKey, nodePublicKey)
	if err != nil {
		return nil, err
	}
	// Derive encryption key
	encryptionKey := DeriveEncryptionKey(sharedSecret[:], []byte("IOEncryptionKeyV1"))
	// Encrypt data
	encryptedData, err := encryptDeoxys(encryptionKey, data)
	if err != nil {
		return nil, err
	}

	// Prepend encrypted data with user public key
	var sizedPrivateKey [32]byte
	copy(sizedPrivateKey[:], privateKey[:32])
	userPublicKey := GetCurve25519PublicKey(sizedPrivateKey)
	return append(userPublicKey[:], encryptedData...), nil
}

func DecryptECDH(privateKey, nodePublicKey, encryptedData []byte) ([]byte, error) {
	// Check length of node public key
	if len(nodePublicKey) != 32 {
		return nil, fmt.Errorf("wrong public key size. Expected 32, got %d", len(nodePublicKey))
	}
	// Derive shared secret
	sharedSecret, err := diffieHellman(privateKey, nodePublicKey)
	if err != nil {
		return nil, err
	}
	// Derive encryption key
	encryptionKey := DeriveEncryptionKey(sharedSecret[:], []byte("IOEncryptionKeyV1"))
	// Decrypt data
	return decryptDeoxys(encryptionKey, encryptedData)
}

func diffieHellman(privateKey, publicKey []byte) ([]byte, error) {
	return curve25519.X25519(privateKey, publicKey)
}

func GetCurve25519PublicKey(privateKey [32]byte) [32]byte {
	var publicKey [32]byte
	curve25519.ScalarBaseMult(&publicKey, &privateKey)
	return publicKey
}

func encryptDeoxys(encryptionKey, plaintext []byte) ([]byte, error) {
	// Generate additional data
	ad := make([]byte, deoxysii.TagSize)
	// Generate random nonce
	var nonce [deoxysii.NonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate random nonce: %w", err)
	}
	// Construct cipher
	cipher, err := deoxysii.New(encryptionKey)
	if err != nil {
		return nil, err
	}
	// Encrypt value
	ciphertext := cipher.Seal(nil, nonce[:], plaintext, ad[:])

	var result = append(nonce[:], ad...)
	result = append(result, ciphertext...)

	return result, nil
}

func decryptDeoxys(encryptionKey, encryptedData []byte) ([]byte, error) {
	// Split encrypted data into nonce, ad, ciphertext
	nonce := encryptedData[:deoxysii.NonceSize]
	ad := encryptedData[deoxysii.NonceSize : deoxysii.NonceSize+deoxysii.TagSize]
	ciphertext := encryptedData[deoxysii.NonceSize+deoxysii.TagSize:]

	// Construct cipher
	cipher, err := deoxysii.New(encryptionKey)
	if err != nil {
		return nil, err
	}

	// Decrypt value
	plaintext, err := cipher.Open(nil, nonce, ciphertext, ad)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// DeriveEncryptionKey derives encryption key using master key and salt
func DeriveEncryptionKey(masterKey, salt []byte) []byte {
	hash := hmac.New(sha256.New, salt)
	hash.Write(masterKey)
	return hash.Sum(nil)
}
