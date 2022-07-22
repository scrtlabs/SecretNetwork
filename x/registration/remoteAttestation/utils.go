package remoteAttestation

import (
	"os"
)

func isSgxHardwareMode() bool {
	return os.Getenv("SGX_MODE") != "SW"
}
