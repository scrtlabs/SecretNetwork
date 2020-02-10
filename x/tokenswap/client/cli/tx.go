package cli

import (
	"bufio"

	"github.com/enigmampc/Enigmachain/x/tokenswap/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/client/utils"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetTxCmd is the CLI command for creating a token swap request
func GetTxCmd(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "create [ethereum-tx-hash] [sender-ethereum-address] [amount-eng] [receiver-enigma-address]",
		Short: "Create a token swap request",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := auth.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))

			ethereumTxHash := args[0]
			ethereumSender := args[1]
			amountENG := args[2]

			receiver, err := sdk.AccAddressFromBech32(args[3])
			if err != nil {
				return err
			}

			msg := types.NewMsgTokenSwap(
				ethereumTxHash,
				ethereumSender,
				receiver,
				amountENG,
			)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}
}
