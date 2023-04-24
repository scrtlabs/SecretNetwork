package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"github.com/scrtlabs/SecretNetwork/x/ibc-switch/types"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Aliases:                    []string{"switch"},
		Short:                      "Ibc-Switch transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	txCmd.AddCommand(
		toggleSwitchCmd(),
	)
	return txCmd
}

// toggleSwitchCmd will toggle the status of the Switch and turn ibc on or off.
func toggleSwitchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "toggle",
		Short: "Toggle the ibc switch on or off",
		Long:  "Toggle the ibc switch on or off. Only a gov-approved address can do this.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgToggleIbcSwitch(clientCtx.GetFromAddress())

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
