package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/cosmos/ibc-go/v4/modules/core/keeper"

	"github.com/scrtlabs/SecretNetwork/x/compute"
)

// HandlerOptions extend the SDK's AnteHandler options by requiring the IBC
// channel keeper.
type HandlerOptions struct {
	ante.HandlerOptions

	IBCKeeper         *keeper.Keeper
	WasmConfig        *compute.WasmConfig
	TXCounterStoreKey sdk.StoreKey
}

func NewAnteHandler(options HandlerOptions) (sdk.AnteHandler, error) {
	if options.HandlerOptions.AccountKeeper == nil {
		return nil, sdkerrors.ErrLogic.Wrap("account keeper is required for ante builder")
	}

	if options.HandlerOptions.BankKeeper == nil {
		return nil, sdkerrors.ErrLogic.Wrap("bank keeper is required for ante builder")
	}

	if options.HandlerOptions.SignModeHandler == nil {
		return nil, sdkerrors.ErrLogic.Wrap("sign mode handler is required for ante builder")
	}

	sigGasConsumer := options.HandlerOptions.SigGasConsumer
	if sigGasConsumer == nil {
		sigGasConsumer = ante.DefaultSigVerificationGasConsumer
	}

	anteDecorators := []sdk.AnteDecorator{
		compute.NewCountTXDecorator(options.TXCounterStoreKey),
		ante.NewSetUpContextDecorator(), // outermost AnteDecorator. SetUpContext must be called first
		ante.NewRejectExtensionOptionsDecorator(),
		ante.NewMempoolFeeDecorator(),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.HandlerOptions.AccountKeeper),
		ante.NewConsumeGasForTxSizeDecorator(options.HandlerOptions.AccountKeeper),
		ante.NewDeductFeeDecorator(options.HandlerOptions.AccountKeeper, options.HandlerOptions.BankKeeper, options.HandlerOptions.FeegrantKeeper),
		ante.NewSetPubKeyDecorator(options.HandlerOptions.AccountKeeper), // SetPubKeyDecorator must be called before all signature verification decorators
		ante.NewValidateSigCountDecorator(options.HandlerOptions.AccountKeeper),
		ante.NewSigGasConsumeDecorator(options.HandlerOptions.AccountKeeper, sigGasConsumer),
		ante.NewSigVerificationDecorator(options.HandlerOptions.AccountKeeper, options.HandlerOptions.SignModeHandler),
		ante.NewIncrementSequenceDecorator(options.HandlerOptions.AccountKeeper),
	}

	return sdk.ChainAnteDecorators(anteDecorators...), nil
}
