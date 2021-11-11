package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/SecretNetwork/x/compute"
	"github.com/enigmampc/SecretNetwork/x/compute/client/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const MessageBlockSize = 256
const flagAmount = "amount"

// S20GetQueryCmd GetQueryCmd returns the cli query commands for this module
func S20GetQueryCmd() *cobra.Command {
	s20QueryCmd := &cobra.Command{
		Use:                        "snip20",
		Short:                      "Querying commands for the secret20 contracts",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	s20QueryCmd.AddCommand(
		S20BalanceCmd(),
		S20TransferHistoryCmd())

	return s20QueryCmd
}

// S20GetTxCmd GetTxCmd returns the transaction commands for this module
func S20GetTxCmd() *cobra.Command {
	s20TxCmd := &cobra.Command{
		Use:                        "snip20",
		Short:                      "Snip20 transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	s20TxCmd.AddCommand(
		s20TransferCmd(),
		s20SendCmd(),
		s20CreatingViewingKey(),
		s20DepositCmd(),
		s20Redeem(),
		s20SetViewingKey(),
		s20BurnCmd())

	return s20TxCmd
}

func S20TransferHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history [contract address] [account] [viewing_key] [optional: page, default: 0] [optional: page_size, default: 10]",
		Short: "View your transaction history",
		Long:  `Print out transactions you have been a part of - either as a sender or recipient`,
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			cliCtx, err := client.GetClientQueryContext(cmd)

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

			var page uint64 = 0
			var pageSize uint64 = 10

			if len(args) == 4 {
				page, err = strconv.ParseUint(args[3], 10, 32)
				if err != nil {
					return err
				}
			}

			if len(args) == 5 {
				pageSize, err = strconv.ParseUint(args[3], 10, 32)
				if err != nil {
					return err
				}
			}

			queryData := queryTransferHistoryMsg(addr, key, uint32(page), uint32(pageSize))

			err = cli.QueryWithData(contractAddr, queryData, cliCtx)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func S20BalanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance [contract address] [account] [viewing_key]",
		Short: "See your current balance for a token",
		Long: `See your current balance for a token. Viewing key must be set for this command to work. If you did not set your viewing 
key yet, use the "create-viewing-key" command. Otherwise, you can still see your current balance using a raw transaction`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			cliCtx, err := client.GetClientQueryContext(cmd)

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

			queryData := queryBalanceMsg(addr, key)

			err = cli.QueryWithData(contractAddr, queryData, cliCtx)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func addressFromBechOrLabel(addressOrLabel string, cliCtx client.Context) (string, error) {
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

func s20TransferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer [contract address or label] [to account] [amount]",
		Short: "Transfer tokens to another address",
		Long:  `transfer tokens to another address`,
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			////inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

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
			if err != nil {
				return errors.New("invalid amount format")
			}

			msg := handleTransferMsg(toAddr, amount)

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cliCtx)
		},
	}

	return cmd
}

func s20CreatingViewingKey() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-viewing-key [contract address or label]",
		Short: "*EXPERIMENTAL* Create a new viewing key. To view the resulting key, use 'secretcli q compute tx <TX_HASH>'",
		Long: `This allows a user to generate a key that enables off-chain queries. 
This way you can perform balance and transaction history queries without waiting for a transaction on-chain.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			//inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

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

			msg := handleCreateViewingKeyMsg(randomData)

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cliCtx)
		},
	}

	return cmd
}

func s20SetViewingKey() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-viewing-key [contract address or label] [viewing-key]",
		Short: "*EXPERIMENTAL* Sets the viewing key for your account",
		Long: `This command is useful if you want to manage multiple secret tokens with the same viewing key. *WARNING*:
This should only be used to duplicate keys created with the create-viewing-key command, or if you really really know what
you're doing`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

			//inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

			contractAddrStr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			contractAddr, err := sdk.AccAddressFromBech32(contractAddrStr)
			if err != nil {
				return errors.New("invalid contract address or label")
			}

			msg := handleSetViewingKeyMsg(args[1])

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cliCtx)
		},
	}

	return cmd
}

func s20DepositCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deposit [contract address or label]",
		Short: "*EXPERIMENTAL* convert your SCRT into a secret token",
		Long:  `Convert your SCRT into a secret token. This command will only work if the token supports native currency conversion`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			//inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

			contractAddrStr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			contractAddr, err := sdk.AccAddressFromBech32(contractAddrStr)
			if err != nil {
				return errors.New("invalid contract address or label")
			}

			msg := handleDepositMsg()

			amountStr := viper.GetString(flagAmount)

			return cli.ExecuteWithData(cmd, contractAddr, msg, amountStr, false, "", "", cliCtx)
		},
	}
	cmd.Flags().String(flagAmount, "", "")
	return cmd
}

func s20Redeem() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "redeem [contract address or label] [amount]",
		Short: "*EXPERIMENTAL* convert your secret token back to SCRT",
		Long:  `Convert your secret token back to SCRT. This command will only work if the token supports native currency conversion`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

			//inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

			contractAddrStr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			contractAddr, err := sdk.AccAddressFromBech32(contractAddrStr)
			if err != nil {
				return errors.New("invalid contract address or label")
			}

			amount := args[1]

			msg := handleRedeemMsg(amount)

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cliCtx)
		},
	}

	return cmd
}

func s20SendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send [contract_address or label] [to_account] [amount] [optional: callback_message]",
		Short: "*EXPERIMENTAL* send tokens to another address. Optionally add a callback message",
		Long: `Send tokens to another address (contract or not). If 'to_account' is a contract, you can optionally add a callback message to this contract.
If no callback provided, this is identical to 'transfer'.`,
		Args: cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			//inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

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
			if err != nil {
				return errors.New("invalid amount format")
			}

			var msg []byte
			if len(args) > 3 {
				callback := args[3]
				msg = handleSendWithCallbackMsg(toAddr, amount, callback)
			} else {
				msg = handleSendMsg(toAddr, amount)
			}

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cliCtx)
		},
	}

	return cmd
}

func s20BurnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burn [contract_address or label] [amount]",
		Short: "*EXPERIMENTAL* burn tokens forever",
		Long: `Burn tokens. The tokens will be removed from your account and will be lost forever.
WARNING! This action is irreversible and permanent! use at your own risk`,
		Args: cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			//inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

			contractAddrStr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			contractAddr, err := sdk.AccAddressFromBech32(contractAddrStr)
			if err != nil {
				return errors.New("invalid contract address or label")
			}

			amount := args[1]
			_, err = strconv.ParseUint(amount, 10, 64)
			if err != nil {
				return errors.New("invalid amount format")
			}

			msg := handleBurnMsg(contractAddr, amount)

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cliCtx)
		},
	}

	return cmd
}

func spacePad(blockSize int, message string) string {
	surplus := len(message) % blockSize
	if surplus == 0 {
		return message
	}

	missing := blockSize - surplus
	return message + strings.Repeat(" ", missing)
}

func queryTransferHistoryMsg(fromAddress sdk.AccAddress, viewingKey string, page uint32, pageSize uint32) []byte {
	return []byte(spacePad(MessageBlockSize,
		fmt.Sprintf("{\"transfer_history\": {\"address\": \"%s\", \"key\": \"%s\", \"page\": %d, \"page_size\": %d}}",
			fromAddress.String(),
			viewingKey,
			page,
			pageSize)))
}

func queryBalanceMsg(fromAddress sdk.AccAddress, viewingKey string) []byte {
	return []byte(spacePad(MessageBlockSize, fmt.Sprintf("{\"balance\": {\"address\": \"%s\", \"key\": \"%s\"}}", fromAddress.String(), viewingKey)))
}

func handleTransferMsg(toAddress sdk.AccAddress, amount string) []byte {
	return []byte(spacePad(MessageBlockSize, fmt.Sprintf("{\"transfer\": {\"recipient\": \"%s\", \"amount\": \"%s\"}}", toAddress.String(), amount)))
}

func handleDepositMsg() []byte {
	return []byte(spacePad(MessageBlockSize, fmt.Sprintf("{\"deposit\": {}}")))
}

func handleRedeemMsg(amount string) []byte {
	return []byte(spacePad(MessageBlockSize, fmt.Sprintf("{\"redeem\": {\"amount\": \"%s\"}}", amount)))
}

func handleCreateViewingKeyMsg(data string) []byte {
	return []byte(spacePad(MessageBlockSize, fmt.Sprintf("{\"create_viewing_key\": {\"entropy\": \"%s\"}}", data)))
}

func handleSetViewingKeyMsg(data string) []byte {
	return []byte(spacePad(MessageBlockSize, fmt.Sprintf("{\"set_viewing_key\": {\"key\": \"%s\"}}", data)))
}

func handleSendWithCallbackMsg(toAddress sdk.AccAddress, amount string, message string) []byte {
	return []byte(spacePad(MessageBlockSize,
		fmt.Sprintf("{\"send\": {\"recipient\": \"%s\", \"amount\": \"%s\", \"msg\": \"%s\"}}",
			toAddress.String(),
			amount,
			message)))
}

func handleSendMsg(toAddress sdk.AccAddress, amount string) []byte {
	return []byte(spacePad(MessageBlockSize, fmt.Sprintf("{\"send\": {\"recipient\": \"%s\", \"amount\": \"%s\"}}", toAddress.String(), amount)))
}

func handleBurnMsg(toAddress sdk.AccAddress, amount string) []byte {
	return []byte(spacePad(MessageBlockSize, fmt.Sprintf("{\"burn\": {\"amount\": \"%s\"}}", amount)))
}
