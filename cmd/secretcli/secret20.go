package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/enigmampc/SecretNetwork/x/compute"
	"github.com/enigmampc/SecretNetwork/x/compute/client/cli"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"math/rand"
	"strconv"
	"time"
)

const flagAmount = "amount"

// GetQueryCmd returns the cli query commands for this module
func S20GetQueryCmd(cdc *codec.Codec) *cobra.Command {
	s20QueryCmd := &cobra.Command{
		Use:                        "secret20",
		Short:                      "*EXPERIMENTAL* Querying commands for the secret20 contracts",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	s20QueryCmd.AddCommand(flags.GetCommands(
		S20BalanceCmd(cdc),
		S20TransferHistoryCmd(cdc),
	)...)

	return s20QueryCmd
}

// GetTxCmd returns the transaction commands for this module
func S20GetTxCmd(cdc *codec.Codec) *cobra.Command {
	s20TxCmd := &cobra.Command{
		Use:                        "secret20",
		Short:                      "*EXPERIMENTAL* Secret20 transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	s20TxCmd.AddCommand(flags.PostCommands(
		s20SendCmd(cdc),
		s20CreatingViewingKey(cdc),
		s20DepositCmd(cdc),
		s20Withdraw(cdc),
		s20SetViewingKey(cdc),
	)...)

	return s20TxCmd
}

func S20TransferHistoryCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history [contract address] [account] [viewing_key]",
		Short: "*EXPERIMENTAL* View your transaction history",
		Long:  `Print out all transactions you have been a part of - either as a sender or recipient`,
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			cliCtx := context.NewCLIContext().WithCodec(cdc)

			contractAddr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			addr, err := sdk.AccAddressFromBech32(args[1])
			if err != nil {
				return err
			}

			key := args[2]
			if key == "" {
				return errors.New("viewing key must not be empty")
			}

			queryData := transferHistoryMsg(addr, key)

			err = cli.QueryWithData(contractAddr, cdc, queryData)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func S20BalanceCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance [contract address] [account] [viewing_key]",
		Short: "*EXPERIMENTAL* See your current balance for a token",
		Long: `See your current balance for a token. Viewing key must be set for this command to work. If you did not set your viewing 
key yet, use the "create-viewing-key" command. Otherwise, you can still see your current balance using a raw transaction`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			cliCtx := context.NewCLIContext().WithCodec(cdc)

			contractAddr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			addr, err := sdk.AccAddressFromBech32(args[1])
			if err != nil {
				return err
			}

			key := args[2]
			if key == "" {
				return errors.New("viewing key must not be empty")
			}

			queryData := balanceMsg(addr, key)

			err = cli.QueryWithData(contractAddr, cdc, queryData)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func addressFromBechOrLabel(addressOrLabel string, cliCtx context.CLIContext) (string, error) {

	var contractAddr string

	_, err := sdk.AccAddressFromBech32(addressOrLabel)
	if err != nil {
		route := fmt.Sprintf("custom/%s/%s/%s", compute.QuerierRoute, compute.QueryContractAddress, addressOrLabel)
		res, _, err := cliCtx.Query(route)
		if err != nil {
			return "", errors.New("requires either contract address or valid label")
		}

		contractAddr = sdk.AccAddress(res).String()
	} else {
		contractAddr = addressOrLabel
	}
	return contractAddr, nil
}

func s20SendCmd(cdc *codec.Codec) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "send [contract address or label] [to account] [amount]",
		Short: "*EXPERIMENTAL* send tokens to another address",
		Long:  `send tokens to another address`,
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			contractAddrStr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			contractAddr, err := sdk.AccAddressFromBech32(contractAddrStr)
			if err != nil {
				return errors.New("invalid contract address or label")
			}

			toAddr, err := sdk.AccAddressFromBech32(args[1])
			if err != nil {
				return errors.New("invalid recipient address")
			}

			amount := args[2]
			_, err = strconv.ParseUint(amount, 10, 64)
			if err == nil {
				return errors.New("invalid amount format")
			}

			msg := sendCoinMsg(toAddr, amount)

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cdc)
		},
	}

	return cmd
}

func s20CreatingViewingKey(cdc *codec.Codec) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "create-viewing-key [contract address or label]",
		Short: "*EXPERIMENTAL* Create a new viewing key. To view the resulting key, use 'secretcli q compute tx <TX_HASH>'",
		Long: `This allows a user to generate a key that enables off-chain queries. 
This way you can perform balance and transaction history queries without waiting for a transaction on-chain. 
This transaction will be expensive, so you must have about 3,000,000 gas in your account to perform this step.
 This is intended to make queries take a long time to execute to be resistant to brute-force attacks.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			contractAddrStr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			contractAddr, err := sdk.AccAddressFromBech32(contractAddrStr)
			if err != nil {
				return errors.New("invalid contract address or label")
			}

			var src = rand.New(rand.NewSource(time.Now().UnixNano()))

			byteArr := make([]byte, 32) // can be simplified to n/2 if n is always even
			if _, err := src.Read(byteArr); err != nil {
				panic(err)
			}

			randomData := hex.EncodeToString(byteArr)[:64]

			msg := createViewingKeyMsg(randomData)

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cdc)
		},
	}

	return cmd
}

func s20SetViewingKey(cdc *codec.Codec) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "set-viewing-key [contract address or label] [viewing-key]",
		Short: "*EXPERIMENTAL* sets the viewing key for your account",
		Long: `This command is useful if you want to manage multiple secret tokens with the same viewing key. *WARNING*:
This should only be used to duplicate keys created with the create-viewing-key command, or if you really really know what
you're doing`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			contractAddrStr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			contractAddr, err := sdk.AccAddressFromBech32(contractAddrStr)
			if err != nil {
				return errors.New("invalid contract address or label")
			}

			msg := setViewingKeyMsg(args[1])

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cdc)
		},
	}

	return cmd
}

func s20DepositCmd(cdc *codec.Codec) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "deposit [contract address or label]",
		Short: "Convert your SCRT into a secret token",
		Long:  `Convert your SCRT into a secret token. This command will only work if the token supports native currency conversion`,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			contractAddrStr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			contractAddr, err := sdk.AccAddressFromBech32(contractAddrStr)
			if err != nil {
				return errors.New("invalid contract address or label")
			}

			msg := depositMsg()

			amountStr := viper.GetString(flagAmount)

			return cli.ExecuteWithData(cmd, contractAddr, msg, amountStr, false, "", "", cdc)
		},
	}
	cmd.Flags().String(flagAmount, "", "")
	return cmd
}

func s20Withdraw(cdc *codec.Codec) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "withdraw [contract address or label] [amount]",
		Short: "Convert your secret token back to SCRT",
		Long:  `Convert your secret token back to SCRT. This command will only work if the token supports native currency conversion`,
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

			inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			contractAddrStr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			contractAddr, err := sdk.AccAddressFromBech32(contractAddrStr)
			if err != nil {
				return errors.New("invalid contract address or label")
			}

			amount := args[1]

			msg := withdrawMsg(amount)

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cdc)
		},
	}

	return cmd
}

func transferHistoryMsg(fromAddress sdk.AccAddress, viewingKey string) []byte {
	return []byte(fmt.Sprintf("{\"transfers\": {\"address\": \"%s\", \"key\": \"%s\"}}", fromAddress.String(), viewingKey))
}

func balanceMsg(fromAddress sdk.AccAddress, viewingKey string) []byte {
	return []byte(fmt.Sprintf("{\"balance\": {\"address\": \"%s\", \"key\": \"%s\"}}", fromAddress.String(), viewingKey))
}

func sendCoinMsg(toAddress sdk.AccAddress, amount string) []byte {
	return []byte(fmt.Sprintf("{\"transfer\": {\"recipient\": \"%s\", \"amount\": \"%s\"}}", toAddress.String(), amount))
}

func depositMsg() []byte {
	return []byte(fmt.Sprintf("{\"deposit\": {}}"))
}

func withdrawMsg(amount string) []byte {
	return []byte(fmt.Sprintf("{\"withdraw\": {\"amount\": \"%s\"}}", amount))
}

func createViewingKeyMsg(data string) []byte {
	return []byte(fmt.Sprintf("{\"create_viewing_key\": {\"entropy\": \"%s\"}}", data))
}

func setViewingKeyMsg(data string) []byte {
	return []byte(fmt.Sprintf("{\"set_viewing_key\": {\"key\": \"%s\"}}", data))
}
