package cli

import (
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Registration transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	txCmd.AddCommand(
		AuthenticateNodeCmd(),
	)
	return txCmd
}

// AuthenticateNodeCmd will upload code to be reused.
func AuthenticateNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth attestation_file [replace_machine_id]",
		Short: "Upload a certificate to authenticate the node",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// clientCtx := client.GetClientContextFromCmd(cmd)
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			cert, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			replace_machine_id := ""

			if len(args) > 1 {
				replace_machine_id = args[1]
			}

			// build and sign the transaction, then broadcast to Tendermint
			msg := types.RaAuthenticate{
				Sender:           clientCtx.GetFromAddress(),
				Certificate:      cert,
				ReplaceMachineId: replace_machine_id,
			}
			err = msg.ValidateBasic()
			if err != nil {
				return xerrors.Errorf("Validtaion on input has failed: %v", err)
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
