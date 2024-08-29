//go:build !nosgx && !attestationServer
// +build !nosgx,!attestationServer

package api

import (
	"fmt"
)

// StartAttestationServer starts attestation server with 2 port (EPID and DCAP attestation)
func StartAttestationServer(epidAddress, dcapAddress string) error {
	fmt.Println("[Attestation Server] Not enabled")
	return nil
}
