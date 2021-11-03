package v120

import (
	v120compute "github.com/enigmampc/SecretNetwork/x/compute/internal/types"
	v106compute "github.com/enigmampc/SecretNetwork/x/compute/legacy/v106"
)

// Migrate accepts exported v1.0.6 x/compute genesis state and
// migrates it to v1.2.0 x/compute genesis state. The migration includes:
//
// - Re-encode in v1.2.0 GenesisState.
func Migrate(computeGenState v106compute.GenesisState) *v120compute.GenesisState {
	codes := make([]v120compute.Code, len(computeGenState.Codes))
	for i, code := range computeGenState.Codes {
		codes[i] = v120compute.Code{
			CodeID: code.CodeID,
			CodeInfo: v120compute.CodeInfo{
				CodeHash: code.CodeInfo.CodeHash,
				Creator:  code.CodeInfo.Creator,
				Source:   code.CodeInfo.Source,
				Builder:  code.CodeInfo.Builder,
			},
			CodeBytes: code.CodesBytes,
		}
	}

	contracts := make([]v120compute.Contract, len(computeGenState.Contracts))
	for i, contract := range computeGenState.Contracts {
		state := make([]v120compute.Model, len(contract.ContractState))
		for j, model := range contract.ContractState {
			state[j] = v120compute.Model{
				Key:   model.Key,
				Value: model.Value,
			}
		}

		var created *v120compute.AbsoluteTxPosition
		if contract.ContractInfo.Created != nil {
			created = &v120compute.AbsoluteTxPosition{
				BlockHeight: contract.ContractInfo.Created.BlockHeight,
				TxIndex:     contract.ContractInfo.Created.TxIndex,
			}
		}

		contracts[i] = v120compute.Contract{
			ContractAddress: contract.ContractAddress,
			ContractInfo: v120compute.ContractInfo{
				CodeID:  contract.ContractInfo.CodeID,
				Creator: contract.ContractInfo.Creator,
				Label:   contract.ContractInfo.Label,
				Created: created,
			},
			ContractState: state,
			ContractCustomInfo: &v120compute.ContractCustomInfo{
				EnclaveKey: contract.ContractCustomInfo.EnclaveKey,
				Label:      contract.ContractCustomInfo.Label,
			},
		}
	}

	sequences := make([]v120compute.Sequence, len(computeGenState.Sequences))
	for i, sequence := range computeGenState.Sequences {
		sequences[i] = v120compute.Sequence{
			IDKey: sequence.IDKey,
			Value: sequence.Value,
		}
	}

	return &v120compute.GenesisState{
		Codes:     codes,
		Contracts: contracts,
		Sequences: sequences,
	}
}
