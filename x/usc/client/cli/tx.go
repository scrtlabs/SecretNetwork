package cli

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/SecretNetwork/x/usc/types"
	"github.com/spf13/cobra"
)

func NewTxCmd() *cobra.Command {
	distTxCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "USC transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	distTxCmd.AddCommand(
		newCmdMintUSC(),
		newCmdRedeemCollateral(),
	)

	return distTxCmd
}

// newCmdQueryParams implements the mint tx command.
func newCmdMintUSC() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mint [amount]",
		Short: "Mint USC coin in exchange for collateral coins (amount)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// Inputs
			fromAddr := clientCtx.GetFromAddress()

			coins, err := sdk.ParseCoinsNormalized(args[0])
			if err != nil {
				return fmt.Errorf("parsing {amount} coins: %w", err)
			}

			// Build and send msg
			msg := types.NewMsgMintUSC(fromAddr, coins)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// newCmdRedeemCollateral implements the redeem tx command.
func newCmdRedeemCollateral() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redeem [amount]",
		Short: "Redeem collateral coins in exchange for USC coin (amount)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// Inputs
			fromAddr := clientCtx.GetFromAddress()

			coin, err := sdk.ParseCoinNormalized(args[0])
			if err != nil {
				return fmt.Errorf("parsing {amount} coin: %w", err)
			}

			// Build and send msg
			msg := types.NewMsgRedeemCollateral(fromAddr, coin)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
