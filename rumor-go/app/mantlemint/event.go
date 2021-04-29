package mantlemint

import (
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/enigmampc/SecretNetwork/rumor-go/types"
	"github.com/enigmampc/SecretNetwork/rumor-go/utils"
)

type EventCollector struct {
	blockState types.BlockState
}

func NewMantlemintEventCollector() *EventCollector {
	return &EventCollector{}
}

func (ev *EventCollector) GetBlockState() *types.BlockState {
	return &ev.blockState
}

// PublishEventNewBlock collects block, ResponseBeginBlock, ResponseEndBlock
func (ev *EventCollector) PublishEventNewBlock(
	block tmtypes.EventDataNewBlock,
) error {
	ev.blockState.Height = block.Block.Height
	ev.blockState.Block = utils.ConvertBlockToRawBlock(block.Block)
	ev.blockState.ResponseBeginBlock = block.ResultBeginBlock
	ev.blockState.ResponseEndBlock = block.ResultEndBlock
	return nil
}

// PublishEventTx collectx txResult in order
func (ev *EventCollector) PublishEventTx(
	txEvent tmtypes.EventDataTx,
) error {
	ev.blockState.ResponseDeliverTx = append(ev.blockState.ResponseDeliverTx, txEvent.TxResult.Result)
	return nil
}

// PublisnEventNewBlockHeader unused
func (ev *EventCollector) PublishEventNewBlockHeader(
	_ tmtypes.EventDataNewBlockHeader,
) error {
	// not implemented
	return nil
}

func (ev *EventCollector) PublishEventValidatorSetUpdates(
	_ tmtypes.EventDataValidatorSetUpdates,
) error {
	// not implemented
	return nil
}
