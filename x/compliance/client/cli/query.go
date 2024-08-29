package cli

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/scrtlabs/SecretNetwork/x/compliance/types"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdQueryParams(),
		CmdGetOperator(),
		CmdGetAddressInfo(),
		CmdGetAddressesInfo(),
		CmdGetIssuerDetails(),
		CmdGetIssuersDetails(),
		CmdGetVerificationDetails(),
		CmdGetVerificationsDetails(),
	)

	return cmd
}

func CmdGetOperator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-operator [bech32-or-hex-address]",
		Short: "Returns operator details associated with provided address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			address, err := types.ParseAddress(args[0])
			if err != nil {
				return err
			}

			req := &types.QueryOperatorDetailsRequest{
				OperatorAddress: address.String(),
			}

			resp, err := queryClient.OperatorDetails(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdGetAddressInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-address-details [bech32-or-hex-address]",
		Short: "Returns address details associated with provided address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			address, err := types.ParseAddress(args[0])
			if err != nil {
				return err
			}

			req := &types.QueryAddressDetailsRequest{
				Address: address.String(),
			}

			resp, err := queryClient.AddressDetails(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdGetAddressesInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-addresses-details",
		Short: "Returns all the address details",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			req := &types.QueryAddressesDetailsRequest{
				Pagination: pageReq,
			}

			resp, err := queryClient.AddressesDetails(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "address details")

	return cmd
}

func CmdGetIssuerDetails() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-issuer-details [bech32-or-hex-address]",
		Short: "Returns issuer details associated with provided address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			address, err := types.ParseAddress(args[0])
			if err != nil {
				return err
			}

			req := &types.QueryIssuerDetailsRequest{
				IssuerAddress: address.String(),
			}

			resp, err := queryClient.IssuerDetails(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdGetIssuersDetails() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-issuers-details",
		Short: "Returns all the issuer details",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			req := &types.QueryIssuersDetailsRequest{
				Pagination: pageReq,
			}

			resp, err := queryClient.IssuersDetails(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "issuer details")

	return cmd
}

func CmdGetVerificationDetails() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-verification-details [verification-id]",
		Short: "Returns verification details associated with provided address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryVerificationDetailsRequest{
				VerificationID: args[0],
			}

			resp, err := queryClient.VerificationDetails(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdGetVerificationsDetails() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-verifications-details",
		Short: "Returns all the verification details",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			req := &types.QueryVerificationsDetailsRequest{
				Pagination: pageReq,
			}

			resp, err := queryClient.VerificationsDetails(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "verification details")

	return cmd
}
