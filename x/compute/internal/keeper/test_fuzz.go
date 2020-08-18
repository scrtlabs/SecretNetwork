package keeper

import (
	"encoding/json"

	"github.com/CosmWasm/wasmd/x/wasm/internal/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	fuzz "github.com/google/gofuzz"
	tmBytes "github.com/tendermint/tendermint/libs/bytes"
)

var ModelFuzzers = []interface{}{FuzzAddr, FuzzAbsoluteTxPosition, FuzzContractInfo, FuzzStateModel, FuzzAccessType, FuzzAccessConfig, FuzzContractCodeHistory}

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
	FuzzAddr(&m.Admin, c)
	m.Label = c.RandString()
	c.Fuzz(&m.Created)
}

func FuzzContractCodeHistory(m *types.ContractCodeHistoryEntry, c fuzz.Continue) {
	const maxMsgSize = 128
	m.CodeID = c.RandUint64()
	msg := make([]byte, c.RandUint64()%maxMsgSize)
	c.Read(msg)
	var err error
	if m.Msg, err = json.Marshal(msg); err != nil {
		panic(err)
	}
	c.Fuzz(&m.Updated)
	m.Operation = types.AllCodeHistoryTypes[c.Int()%len(types.AllCodeHistoryTypes)]
}

func FuzzStateModel(m *types.Model, c fuzz.Continue) {
	m.Key = tmBytes.HexBytes(c.RandString())
	c.Fuzz(&m.Value)
}

func FuzzAccessType(m *types.AccessType, c fuzz.Continue) {
	pos := c.Int() % len(types.AllAccessTypes)
	for k, _ := range types.AllAccessTypes {
		if pos == 0 {
			*m = k
			return
		}
		pos--
	}
}
func FuzzAccessConfig(m *types.AccessConfig, c fuzz.Continue) {
	FuzzAccessType(&m.Type, c)
	var add sdk.AccAddress
	FuzzAddr(&add, c)
	*m = m.Type.With(add)
}
