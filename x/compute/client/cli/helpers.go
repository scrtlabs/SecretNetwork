package cli

import "fmt"

func parseEncryptedBlob(blob []byte) ([]byte, []byte, []byte, error) {
	if len(blob) < 64 {
		return nil, nil, nil, fmt.Errorf("input must be > 64 bytes. Got %d", len(blob))
	}

	nonce := blob[0:32]
	originalTxSenderPubkey := blob[32:64]
	ciphertextInput := blob[64:]

	return nonce, originalTxSenderPubkey, ciphertextInput, nil
}
