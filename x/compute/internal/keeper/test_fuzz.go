package keeper

import (
	tmBytes "github.com/cometbft/cometbft/libs/bytes"
	sdk "github.com/cosmos/cosmos-sdk/types"
	fuzz "github.com/google/gofuzz"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

var ModelFuzzers = []interface{}{FuzzAddr, FuzzAbsoluteTxPosition, FuzzContractInfo, FuzzStateModel}

func FuzzAddr(m *sdk.AccAddress, c fuzz.Continue) {
	*m = make([]byte, 20)
	c.Read(*m)
}

func FuzzAbsoluteTxPosition(m *types.AbsoluteTxPosition, c fuzz.Continue) {
	m.BlockHeight = int64(c.RandUint64()) // can't be negative
	m.TxIndex = c.RandUint64()
}

func FuzzContractInfo(m *types.ContractInfo, c fuzz.Continue) {
	m.CodeID = c.RandUint64()
	FuzzAddr(&m.Creator, c)
	m.Label = c.RandString()
	c.Fuzz(&m.Created)
}

func FuzzStateModel(m *types.Model, c fuzz.Continue) {
	m.Key = tmBytes.HexBytes(c.RandString())
	c.Fuzz(&m.Value)
}
