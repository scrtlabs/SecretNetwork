package keeper

import (
	"sync"
)

// DEPRECATED: Disk-based key persistence has been removed.
// Key shares are now stored encrypted on-chain and loaded on-demand for signing.
// This file is kept for backwards compatibility and may be removed in future versions.

var (
	// nodeHome is kept for compatibility but no longer used for key storage
	nodeHome     string
	nodeHomeLock sync.RWMutex
)

// SetNodeHome is kept for backwards compatibility but no longer stores keys to disk
// Keys are now stored encrypted on-chain
func SetNodeHome(home string) {
	nodeHomeLock.Lock()
	defer nodeHomeLock.Unlock()
	nodeHome = home
}

// LoadFROSTKeyShares is a no-op for backwards compatibility
// Keys are now loaded on-demand from on-chain encrypted storage
func LoadFROSTKeyShares() error {
	// No longer loading from disk - keys are stored encrypted on-chain
	// and loaded on-demand when signing is needed
	return nil
}
