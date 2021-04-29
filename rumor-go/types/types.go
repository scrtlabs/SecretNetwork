package types

import (
	tmtypes "github.com/tendermint/tendermint/types"
)

// TODO: move this part to mantle-compatibility
type (
	Tx         = tmtypes.Tx
	GenesisDoc = tmtypes.GenesisDoc
	Block      = tmtypes.Block
	Header     = tmtypes.Header

	RawBlock struct {
		Header     tmtypes.Header       `json:"header"`
		Data       tmtypes.Data         `json:"data"`
		Evidence   tmtypes.EvidenceData `json:"evidence"`
		LastCommit tmtypes.Commit       `json:"last_commit"`
	}
)
