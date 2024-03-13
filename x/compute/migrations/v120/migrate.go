package v120

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gogo/protobuf/proto"
	v120compute "github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
	v106compute "github.com/scrtlabs/SecretNetwork/x/compute/migrations/v106"
)

// Migrate accepts exported v1.0.6 x/compute genesis state and
// migrates it to v1.2.0 x/compute genesis state. The migration includes:
//
// - Re-encode in v1.2.0 GenesisState.
func Migrate(computeGenState v106compute.GenesisState) *V1GenesisState {
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

	contracts := make([]v1Contract, len(computeGenState.Contracts))
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

		contracts[i] = v1Contract{
			ContractAddress: contract.ContractAddress,
			ContractInfo: v120compute.ContractInfo{
				CodeID:  contract.ContractInfo.CodeID,
				Creator: contract.ContractInfo.Creator,
				Label:   contract.ContractInfo.Label,
				Created: created,
			},
			ContractState: state,
			ContractCustomInfo: &v1ContractCustomInfo{
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

	return &V1GenesisState{
		Codes:     codes,
		Contracts: contracts,
		Sequences: sequences,
	}
}

type v1ContractCustomInfo struct {
	EnclaveKey []byte `protobuf:"bytes,1,opt,name=enclave_key,json=enclaveKey,proto3" json:"enclave_key,omitempty"`
	Label      string `protobuf:"bytes,2,opt,name=label,proto3" json:"label,omitempty"`
}

type v1Contract struct {
	ContractAddress    sdk.AccAddress           `protobuf:"bytes,1,opt,name=contract_address,json=contractAddress,proto3,casttype=github.com/cosmos/cosmos-sdk/types.AccAddress" json:"contract_address,omitempty"`
	ContractInfo       v120compute.ContractInfo `protobuf:"bytes,2,opt,name=contract_info,json=contractInfo,proto3" json:"contract_info"`
	ContractState      []v120compute.Model      `protobuf:"bytes,3,rep,name=contract_state,json=contractState,proto3" json:"contract_state"`
	ContractCustomInfo *v1ContractCustomInfo    `protobuf:"bytes,4,opt,name=contract_custom_info,json=contractCustomInfo,proto3" json:"contract_custom_info,omitempty"`
}

// V1GenesisState - genesis state of x/wasm
type V1GenesisState struct {
	//    Params params = 1 [(gogoproto.nullable) = false];
	Codes     []v120compute.Code     `protobuf:"bytes,2,rep,name=codes,proto3" json:"codes,omitempty"`
	Contracts []v1Contract           `protobuf:"bytes,3,rep,name=contracts,proto3" json:"contracts,omitempty"`
	Sequences []v120compute.Sequence `protobuf:"bytes,4,rep,name=sequences,proto3" json:"sequences,omitempty"`
}

func (m *V1GenesisState) Reset()         { *m = V1GenesisState{} }
func (m *V1GenesisState) String() string { return proto.CompactTextString(m) }
func (*V1GenesisState) ProtoMessage()    {}
