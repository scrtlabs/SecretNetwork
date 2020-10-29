package client

import (
	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/enigmampc/SecretNetwork/x/tokenswap/client/cli"
	"github.com/enigmampc/SecretNetwork/x/tokenswap/client/rest"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client/flags"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd(storeKey string, cdc *codec.Codec) *cobra.Command {
	// Group tokenswap queries under a subcommand
	tokenSwapQueryCmd := &cobra.Command{
		Use:   "tokenswap",
		Short: "Querying commands for the tokenswap module",
	}

	tokenSwapQueryCmd.AddCommand(flags.GetCommands(
		cli.GetQueryCmd(storeKey, cdc),
	)...)

	return tokenSwapQueryCmd
}

// GetTxCmd returns the transaction commands for this module
func GetTxCmd(storeKey string, cdc *codec.Codec) *cobra.Command {
	tokenSwapTxCmd := &cobra.Command{
		Use:   "tokenswap",
		Short: "tokenswap transactions subcommands",
	}

	tokenSwapTxCmd.AddCommand(flags.PostCommands(
		cli.GetTxCmd(cdc),
	)...)

	return tokenSwapTxCmd
}

// RegisterRESTRoutes - Central function to define routes that get registered by the main application
func RegisterRESTRoutes(cliCtx context.CLIContext, r *mux.Router, storeName string) {
	rest.RegisterRESTRoutes(cliCtx, r, storeName)
}
