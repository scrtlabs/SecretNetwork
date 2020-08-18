package types

import (
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestContractInfoValidateBasic(t *testing.T) {
	specs := map[string]struct {
		srcMutator func(*ContractInfo)
		expError   bool
	}{
		"all good": {srcMutator: func(_ *ContractInfo) {}},
		"code id empty": {
			srcMutator: func(c *ContractInfo) { c.CodeID = 0 },
			expError:   true,
		},
		"creator empty": {
			srcMutator: func(c *ContractInfo) { c.Creator = nil },
			expError:   true,
		},
		"creator not an address": {
			srcMutator: func(c *ContractInfo) { c.Creator = make([]byte, sdk.AddrLen-1) },
			expError:   true,
		},
		"admin empty": {
			srcMutator: func(c *ContractInfo) { c.Admin = nil },
			expError:   false,
		},
		"admin not an address": {
			srcMutator: func(c *ContractInfo) { c.Admin = make([]byte, sdk.AddrLen-1) },
			expError:   true,
		},
		"label empty": {
			srcMutator: func(c *ContractInfo) { c.Label = "" },
			expError:   true,
		},
		"label exceeds limit": {
			srcMutator: func(c *ContractInfo) { c.Label = strings.Repeat("a", MaxLabelSize+1) },
			expError:   true,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			state := ContractInfoFixture(spec.srcMutator)
			got := state.ValidateBasic()
			if spec.expError {
				require.Error(t, got)
				return
			}
			require.NoError(t, got)
		})
	}
}

func TestCodeInfoValidateBasic(t *testing.T) {
	specs := map[string]struct {
		srcMutator func(*CodeInfo)
		expError   bool
	}{
		"all good": {srcMutator: func(_ *CodeInfo) {}},
		"code hash empty": {
			srcMutator: func(c *CodeInfo) { c.CodeHash = []byte{} },
			expError:   true,
		},
		"code hash nil": {
			srcMutator: func(c *CodeInfo) { c.CodeHash = nil },
			expError:   true,
		},
		"creator empty": {
			srcMutator: func(c *CodeInfo) { c.Creator = nil },
			expError:   true,
		},
		"creator not an address": {
			srcMutator: func(c *CodeInfo) { c.Creator = make([]byte, sdk.AddrLen-1) },
			expError:   true,
		},
		"source empty": {
			srcMutator: func(c *CodeInfo) { c.Source = "" },
		},
		"source not an url": {
			srcMutator: func(c *CodeInfo) { c.Source = "invalid" },
			expError:   true,
		},
		"source not an absolute url": {
			srcMutator: func(c *CodeInfo) { c.Source = "../bar.txt" },
			expError:   true,
		},
		"source not https schema url": {
			srcMutator: func(c *CodeInfo) { c.Source = "http://example.com" },
			expError:   true,
		},
		"builder tag exceeds limit": {
			srcMutator: func(c *CodeInfo) { c.Builder = strings.Repeat("a", MaxBuildTagSize+1) },
			expError:   true,
		},
		"builder tag does not match pattern": {
			srcMutator: func(c *CodeInfo) { c.Builder = "invalid" },
			expError:   true,
		},
		"Instantiate config invalid": {
			srcMutator: func(c *CodeInfo) { c.InstantiateConfig = AccessConfig{} },
			expError:   true,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			state := CodeInfoFixture(spec.srcMutator)
			got := state.ValidateBasic()
			if spec.expError {
				require.Error(t, got)
				return
			}
			require.NoError(t, got)
		})
	}
}
