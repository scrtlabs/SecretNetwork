package remoteAttestation

import (
	"os"
)

func isSgxHardwareMode() bool {
	if os.Getenv("SGX_MODE") != "SW" {
		return true
	}
	return false
}
