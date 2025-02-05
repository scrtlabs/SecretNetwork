package keeper

import (
	"crypto/rand"
	"fmt"
	"os"
	"strconv"

	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
)

const SEED_SIZE = types.EncryptedKeyLength / 2

func isSimulationAllowed() bool {
	if os.Getenv("SGX_MODE") == "SW" {
		docker := os.Getenv("SGX_DOCKER_RUN")
		if len(docker) > 0 {
			magic, err := strconv.Atoi(docker)
			if err != nil {
				return false
			}
			return magic != 0 && magic%43 == 0
		}
	}
	return false
}

func backup_SEED() ([]byte, error) {
	var encSeed []byte
	if isSimulationAllowed() {
		encSeed = make([]byte, SEED_SIZE)
		if n, _ := rand.Read(encSeed); n == SEED_SIZE {
			return encSeed, nil
		}
	}
	return nil, fmt.Errorf("simulation is not allowed")
}
