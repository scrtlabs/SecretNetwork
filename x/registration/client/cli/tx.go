package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/enigmampc/SecretNetwork/x/registration/internal/types"
	"github.com/spf13/cobra"
	"io/ioutil"
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
		Use:   "auth [cert file]",
		Short: "Upload a certificate to authenticate the node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			//clientCtx := client.GetClientContextFromCmd(cmd)
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			cert, err := ioutil.ReadFile(args[0])
			if err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			msg := types.RaAuthenticate{
				Sender:      clientCtx.GetFromAddress(),
				Certificate: cert,
			}
			err = msg.ValidateBasic()

			if err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
