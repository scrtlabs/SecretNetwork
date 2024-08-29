package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/gov/client/cli"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/spf13/cobra"

	"github.com/scrtlabs/SecretNetwork/x/compliance/types"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Compliance transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdAddOperator(),
		CmdRemoveOperator(),
		CmdSetIssuerVerificationStatus(),
		CmdCreateIssuer(),
		CmdUpdateIssuerDetails(),
		CmdRemoveIssuer(),
	)

	return cmd
}

// CmdAddOperator command adds regular operator.
func CmdAddOperator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-operator [operator-address]",
		Short: "Add regular operator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			operator, err := types.ParseAddress(args[0])
			if err != nil {
				return err
			}

			msg := types.NewMsgAddOperator(
				clientCtx.GetFromAddress().String(),
				operator.String(),
			)

			_ = clientCtx.PrintProto(&msg)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdRemoveOperator command removes regular operator.
func CmdRemoveOperator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-operator [operator-address]",
		Short: "Remove regular operator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			operator, err := types.ParseAddress(args[0])
			if err != nil {
				return err
			}

			msg := types.NewMsgRemoveOperator(
				clientCtx.GetFromAddress().String(),
				operator.String(),
			)

			_ = clientCtx.PrintProto(&msg)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdSetIssuerVerificationStatus command set issuer's verification status with given parameter.
func CmdSetIssuerVerificationStatus() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-issuer-status [issuer-address] [verification-status]",
		Short: "Set issuer's verification status",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			issuer, err := types.ParseAddress(args[0])
			if err != nil {
				return err
			}
			isVerified, err := strconv.ParseBool(args[1])
			if err != nil {
				return err
			}

			msg := types.NewMsgSetVerificationStatus(
				clientCtx.GetFromAddress().String(),
				issuer.String(),
				isVerified,
			)

			_ = clientCtx.PrintProto(&msg)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdCreateIssuer command creates issuer with provided details.
func CmdCreateIssuer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-issuer [issuer-address] [name] [description] [url] [logo-url] [legal-entity]",
		Short: "Create issuer with details",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			issuerAddress, err := types.ParseAddress(args[0])
			if err != nil {
				return err
			}

			issuerName := args[1]
			issuerDescription := args[2]
			issuerURL := args[3]
			issuerLogo := args[4]
			issuerLegalEntity := args[5]

			msg := types.NewCreateIssuerMsg(
				clientCtx.GetFromAddress().String(),
				issuerAddress.String(),
				issuerName,
				issuerDescription,
				issuerURL,
				issuerLogo,
				issuerLegalEntity,
			)

			_ = clientCtx.PrintProto(&msg)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdUpdateIssuerDetails command updates existing issuer details.
func CmdUpdateIssuerDetails() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-issuer-details [issuer-address] [name] [description] [url] [logo-url] [legal-entity]",
		Short: "Update issuer details",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			issuerAddress, err := types.ParseAddress(args[0])
			if err != nil {
				return err
			}

			issuerName := args[1]
			issuerDescription := args[2]
			issuerURL := args[3]
			issuerLogo := args[4]
			issuerLegalEntity := args[5]

			msg := types.NewUpdateIssuerDetailsMsg(
				clientCtx.GetFromAddress().String(),
				issuerAddress.String(),
				issuerName,
				issuerDescription,
				issuerURL,
				issuerLogo,
				issuerLegalEntity,
			)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdRemoveIssuer command removes existing issuer.
func CmdRemoveIssuer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-issuer [issuer-address]",
		Short: "Removes existing issuer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			issuerAddress, err := types.ParseAddress(args[0])
			if err != nil {
				return err
			}

			msg := types.NewRemoveIssuerMsg(
				clientCtx.GetFromAddress().String(),
				issuerAddress.String(),
			)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func CmdVerifyIssuerProposal() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "verify-issuer [issuer-address]",
		Args:    cobra.MinimumNArgs(1),
		Short:   "Submit a proposal to verify issuer",
		Long:    "Submit a proposal to verify issuer along with an initial deposit.",
		Example: fmt.Sprintf("$ %s tx gov submit-legacy-proposal verify-issuer <issuer address>", version.AppName),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			title, err := cmd.Flags().GetString(cli.FlagTitle)
			if err != nil {
				return err
			}

			description, err := cmd.Flags().GetString(cli.FlagDescription) //nolint:staticcheck
			if err != nil {
				return err
			}

			depositStr, err := cmd.Flags().GetString(cli.FlagDeposit)
			if err != nil {
				return err
			}

			deposit, err := sdk.ParseCoinsNormalized(depositStr)
			if err != nil {
				return err
			}

			issuerAddress := args[0]
			from := clientCtx.GetFromAddress()

			// Verified issuer can't create proposal
			queryClient := types.NewQueryClient(clientCtx)
			addressDetails, err := queryClient.AddressDetails(cmd.Context(), &types.QueryAddressDetailsRequest{
				Address: issuerAddress,
			})
			if err != nil {
				return err
			}
			if addressDetails.Data.IsVerified {
				return errors.New("issuer was already verified")
			}

			content := types.NewVerifyIssuerProposal(title, description, issuerAddress)

			msg, err := govv1beta1.NewMsgSubmitProposal(content, deposit, from)
			if err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String(cli.FlagTitle, "", "title of proposal")
	cmd.Flags().String(cli.FlagDescription, "", "description of proposal")
	cmd.Flags().String(cli.FlagDeposit, "1aswtr", "deposit of proposal")
	if err := cmd.MarkFlagRequired(cli.FlagTitle); err != nil {
		panic(err)
	}
	if err := cmd.MarkFlagRequired(cli.FlagDescription); err != nil {
		panic(err)
	}
	if err := cmd.MarkFlagRequired(cli.FlagDeposit); err != nil {
		panic(err)
	}
	return cmd
}
