package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/scrtlabs/SecretNetwork/x/mauth/types"
	"github.com/spf13/cobra"
)

// GetQueryCmd creates and returns the intertx query command
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the mauth module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(getInterchainAccountCmd())

	return cmd
}

func getInterchainAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "interchainaccounts [account] [connection-id] [counterparty-connection-id]",
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.InterchainAccountFromAddress(cmd.Context(), types.NewQueryInterchainAccountRequest(args[0], args[1], args[2]))
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
