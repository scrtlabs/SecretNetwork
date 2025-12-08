package cli

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

// GetQueryCmd returns the cli query commands for the module
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetCmdQueryParams(),
		GetCmdQueryKeySet(),
		GetCmdQueryAllKeySets(),
		GetCmdQueryDKGSession(),
		GetCmdQueryAllDKGSessions(),
		GetCmdQuerySigningRequest(),
		GetCmdQueryAllSigningRequests(),
	)

	return cmd
}

// GetCmdQueryParams implements the params query command
func GetCmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Shows the parameters of the module",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.Params(context.Background(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdQueryKeySet implements the key-set query command
func GetCmdQueryKeySet() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key-set [id]",
		Short: "Query a KeySet by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.KeySet(context.Background(), &types.QueryKeySetRequest{
				Id: args[0],
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdQueryAllKeySets implements the all-key-sets query command
func GetCmdQueryAllKeySets() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all-key-sets",
		Short: "Query all KeySets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.AllKeySets(context.Background(), &types.QueryAllKeySetsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdQueryDKGSession implements the dkg-session query command
func GetCmdQueryDKGSession() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dkg-session [session-id]",
		Short: "Query a DKG session by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.DKGSession(context.Background(), &types.QueryDKGSessionRequest{
				SessionId: args[0],
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdQueryAllDKGSessions implements the all-dkg-sessions query command
func GetCmdQueryAllDKGSessions() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all-dkg-sessions",
		Short: "Query all DKG sessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.AllDKGSessions(context.Background(), &types.QueryAllDKGSessionsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdQuerySigningRequest implements the signing-request query command
func GetCmdQuerySigningRequest() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "signing-request [request-id]",
		Short: "Query a signing request by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.SigningRequest(context.Background(), &types.QuerySigningRequestRequest{
				RequestId: args[0],
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdQueryAllSigningRequests implements the all-signing-requests query command
func GetCmdQueryAllSigningRequests() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all-signing-requests",
		Short: "Query all signing requests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.AllSigningRequests(context.Background(), &types.QueryAllSigningRequestsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
