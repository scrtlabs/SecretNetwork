package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/SecretNetwork/x/compute"
	"github.com/enigmampc/SecretNetwork/x/compute/client/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"strconv"
	"strings"
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
		S20TransferHistoryCmd(),
		S20TransactionHistoryCmd(),
	)

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
		s20BurnCmd(),
	)

	return s20TxCmd
}

func S20TransferHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfers [contract address] [account] [viewing_key] [optional: page, default: 0] [optional: page_size, default: 10]",
		Short: "View your transfer history",
		Long:  `Print out transfer you have been a part of - either as a sender or recipient`,
		Args:  cobra.RangeArgs(3, 5),
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

			if len(args) >= 4 {
				page, err = strconv.ParseUint(args[3], 10, 32)
				if err != nil {
					return err
				}
			}

			if len(args) == 5 {
				pageSize, err = strconv.ParseUint(args[4], 10, 32)
				if err != nil {
					return err
				}
			}

			queryData, err := queryTransferHistoryMsg(addr, key, uint32(page), uint32(pageSize))
			if err != nil {
				return err
			}

			err = cli.QueryWithData(contractAddr, queryData, cliCtx)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func S20TransactionHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "txs [contract address] [account] [viewing_key] [optional: page, default: 0] [optional: page_size, default: 10]",
		Short: "View your full transaction history",
		Long: `Print out transactions you have been a part of - either as a sender or recipient.
Unlike the transfers query, this query shows all kinds of transactions with the contract.`,
		Args: cobra.RangeArgs(3, 5),
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

			if len(args) >= 4 {
				page, err = strconv.ParseUint(args[3], 10, 32)
				if err != nil {
					return err
				}
			}

			if len(args) == 5 {
				pageSize, err = strconv.ParseUint(args[4], 10, 32)
				if err != nil {
					return err
				}
			}

			queryData, err := queryTransactionHistoryMsg(addr, key, uint32(page), uint32(pageSize))
			if err != nil {
				return err
			}

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

			queryData, err := queryBalanceMsg(addr, key)
			if err != nil {
				return err
			}

			err = cli.QueryWithData(contractAddr, queryData, cliCtx)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func addressFromBechOrLabel(addressOrLabel string, cliCtx client.Context) (sdk.AccAddress, error) {
	contractAddr, err := sdk.AccAddressFromBech32(addressOrLabel)
	if err != nil {
		route := fmt.Sprintf("custom/%s/%s/%s", compute.QuerierRoute, compute.QueryContractAddress, addressOrLabel)
		res, _, err := cliCtx.Query(route)
		if err != nil {
			return sdk.AccAddress{}, errors.New("requires either contract address or valid label")
		}

		// We assume that the query above returns a valid address
		contractAddr = res
	}
	return contractAddr, nil
}

func s20TransferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer [contract address or label] [to account] [amount]",
		Short: "Transfer tokens to another address",
		Long:  `Transfer tokens to another address`,
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			////inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

			contractAddr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
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

			msg, err := handleTransferMsg(toAddr, amount)
			if err != nil {
				return err
			}

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cliCtx)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func s20CreatingViewingKey() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-viewing-key [contract address or label]",
		Short: "Create a new viewing key. To view the resulting key, use 'secretcli q compute tx <TX_HASH>'",
		Long: `This allows a user to generate a key that enables off-chain queries. 
This way you can perform balance and transaction history queries without waiting for a transaction on-chain.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			//inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

			contractAddr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			byteArr := make([]byte, 32) // can be simplified to n/2 if n is always even
			if _, err := rand.Read(byteArr); err != nil {
				panic(err)
			}

			randomData := hex.EncodeToString(byteArr)[:64]

			msg, err := handleCreateViewingKeyMsg(randomData)
			if err != nil {
				return err
			}

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cliCtx)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func s20SetViewingKey() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-viewing-key [contract address or label] [viewing-key]",
		Short: "Sets the viewing key for your account",
		Long: `This command is useful if you want to manage multiple secret tokens with the same viewing key.
*WARNING*: This should only be used to duplicate keys created with the create-viewing-key command, or if you really really know what
you're doing`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

			//inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

			contractAddr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			msg, err := handleSetViewingKeyMsg(args[1])
			if err != nil {
				return err
			}

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cliCtx)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func s20DepositCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deposit [contract address or label]",
		Short: "Convert your SCRT into a secret token",
		Long:  `Convert your SCRT into a secret token. This command will only work if the token supports native currency conversion`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			//inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

			contractAddr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			msg, err := handleDepositMsg()
			if err != nil {
				return err
			}

			amountStr := viper.GetString(flagAmount)

			return cli.ExecuteWithData(cmd, contractAddr, msg, amountStr, false, "", "", cliCtx)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(flagAmount, "", "The amount of currency to deposit in the contract, e.g. 1000000uscrt")
	_ = cmd.MarkFlagRequired(flagAmount)

	return cmd
}

func s20Redeem() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redeem [contract address or label] [amount]",
		Short: "Convert your secret token back to SCRT",
		Long:  `Convert your secret token back to SCRT. This command will only work if the token supports native currency conversion`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

			//inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

			contractAddr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			amount := args[1]
			_, err = strconv.ParseUint(amount, 10, 64)
			if err != nil {
				return errors.New("invalid amount format")
			}

			msg, err := handleRedeemMsg(amount)
			if err != nil {
				return err
			}

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cliCtx)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func s20SendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send [contract_address or label] [to_account] [amount] [optional: callback_message]",
		Short: "Send tokens to another address. Optionally add a callback message",
		Long: `Send tokens to another address (contract or not). If 'to_account' is a contract, you can optionally add a callback message to this contract.
If no callback provided, this is identical to 'transfer'.`,
		Args: cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {

			//inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

			contractAddr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
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

			var callback string
			if len(args) > 3 {
				callback = args[3]
			}
			msg, err := handleSendMsg(toAddr, amount, callback)
			if err != nil {
				return err
			}

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cliCtx)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func s20BurnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burn [contract_address or label] [amount]",
		Short: "Burn tokens forever",
		Long: `Burn tokens. The tokens will be removed from your account and will be lost forever.
WARNING! This action is irreversible and permanent! use at your own risk`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

			//inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx, err := client.GetClientTxContext(cmd)

			contractAddr, err := addressFromBechOrLabel(args[0], cliCtx)
			if err != nil {
				return err
			}

			amount := args[1]
			_, err = strconv.ParseUint(amount, 10, 64)
			if err != nil {
				return errors.New("invalid amount format")
			}

			msg, err := handleBurnMsg(amount)
			if err != nil {
				return err
			}

			return cli.ExecuteWithData(cmd, contractAddr, msg, "", false, "", "", cliCtx)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

type TransferHistoryMsg struct {
	TransferHistory TransferHistoryMsgInner `json:"transfer_history"`
}

type TransferHistoryMsgInner struct {
	Address  sdk.AccAddress `json:"address"`
	Key      string         `json:"key"`
	Page     uint32         `json:"page"`
	PageSize uint32         `json:"page_size"`
}

type TransactionHistoryMsg struct {
	TransactionHistory TransactionHistoryMsgInner `json:"transaction_history"`
}

type TransactionHistoryMsgInner struct {
	Address  sdk.AccAddress `json:"address"`
	Key      string         `json:"key"`
	Page     uint32         `json:"page"`
	PageSize uint32         `json:"page_size"`
}

type BalanceMsg struct {
	Balance BalanceMsgInner `json:"balance"`
}

type BalanceMsgInner struct {
	Address sdk.AccAddress `json:"address"`
	Key     string         `json:"key"`
}

type TransferMsg struct {
	Transfer TransferMsgInner `json:"transfer"`
}

type TransferMsgInner struct {
	Recipient sdk.AccAddress `json:"recipient"`
	Amount    string         `json:"amount"`
}

type CreateViewingKeyMsg struct {
	CreateViewingKey CreateViewingKeyMsgInner `json:"create_viewing_key"`
}

type CreateViewingKeyMsgInner struct {
	Entropy string `json:"entropy"`
}

type SetViewingKeyMsg struct {
	SetViewingKey SetViewingKeyMsgInner `json:"set_viewing_key"`
}

type SetViewingKeyMsgInner struct {
	Key string `json:"key"`
}

type DepositMsg struct {
	Deposit DepositMsgInner `json:"deposit"`
}

type DepositMsgInner struct{}

type RedeemMsg struct {
	Redeem RedeemMsgInner `json:"redeem"`
}

type RedeemMsgInner struct {
	Amount string `json:"amount"`
}

type SendMsg struct {
	Send SendMsgInner `json:"send"`
}

type SendMsgInner struct {
	Recipient sdk.AccAddress `json:"recipient"`
	Amount    string         `json:"amount"`
	Msg       string         `json:"msg,omitempty"`
}

type BurnMsg struct {
	Burn BurnMsgInner `json:"burn"`
}

type BurnMsgInner struct {
	Amount string `json:"amount"`
}

func spacePad(blockSize int, message string) string {
	surplus := len(message) % blockSize
	if surplus == 0 {
		return message
	}

	missing := blockSize - surplus
	return message + strings.Repeat(" ", missing)
}

func queryTransferHistoryMsg(fromAddress sdk.AccAddress, viewingKey string, page uint32, pageSize uint32) ([]byte, error) {
	msg := TransferHistoryMsg{
		TransferHistory: TransferHistoryMsgInner{
			Address:  fromAddress,
			Key:      viewingKey,
			Page:     page,
			PageSize: pageSize,
		},
	}
	jsonMsg, err := json.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	return []byte(spacePad(MessageBlockSize, string(jsonMsg))), nil
}

func queryTransactionHistoryMsg(fromAddress sdk.AccAddress, viewingKey string, page uint32, pageSize uint32) ([]byte, error) {
	msg := TransactionHistoryMsg{
		TransactionHistory: TransactionHistoryMsgInner{
			Address:  fromAddress,
			Key:      viewingKey,
			Page:     page,
			PageSize: pageSize,
		},
	}
	jsonMsg, err := json.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	return []byte(spacePad(MessageBlockSize, string(jsonMsg))), nil
}

func queryBalanceMsg(fromAddress sdk.AccAddress, viewingKey string) ([]byte, error) {
	msg := BalanceMsg{
		Balance: BalanceMsgInner{
			Address: fromAddress,
			Key:     viewingKey,
		},
	}
	jsonMsg, err := json.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	return []byte(spacePad(MessageBlockSize, string(jsonMsg))), nil
}

func handleTransferMsg(toAddress sdk.AccAddress, amount string) ([]byte, error) {
	msg := TransferMsg{
		Transfer: TransferMsgInner{
			Recipient: toAddress,
			Amount:    amount,
		},
	}
	jsonMsg, err := json.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	return []byte(spacePad(MessageBlockSize, string(jsonMsg))), nil
}

func handleCreateViewingKeyMsg(entropy string) ([]byte, error) {
	msg := CreateViewingKeyMsg{
		CreateViewingKey: CreateViewingKeyMsgInner{
			Entropy: entropy,
		},
	}
	jsonMsg, err := json.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	return []byte(spacePad(MessageBlockSize, string(jsonMsg))), nil
}

func handleSetViewingKeyMsg(key string) ([]byte, error) {
	msg := SetViewingKeyMsg{
		SetViewingKey: SetViewingKeyMsgInner{
			Key: key,
		},
	}
	jsonMsg, err := json.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	return []byte(spacePad(MessageBlockSize, string(jsonMsg))), nil
}

func handleDepositMsg() ([]byte, error) {
	msg := DepositMsg{Deposit: DepositMsgInner{}}
	jsonMsg, err := json.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	return []byte(spacePad(MessageBlockSize, string(jsonMsg))), nil
}

func handleRedeemMsg(amount string) ([]byte, error) {
	msg := RedeemMsg{
		Redeem: RedeemMsgInner{
			Amount: amount,
		},
	}
	jsonMsg, err := json.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	return []byte(spacePad(MessageBlockSize, string(jsonMsg))), nil
}

func handleSendMsg(toAddress sdk.AccAddress, amount string, message string) ([]byte, error) {
	msg := SendMsg{
		Send: SendMsgInner{
			Recipient: toAddress,
			Amount:    amount,
			Msg:       message,
		},
	}
	jsonMsg, err := json.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	return []byte(spacePad(MessageBlockSize, string(jsonMsg))), nil
}

func handleBurnMsg(amount string) ([]byte, error) {
	msg := BurnMsg{
		Burn: BurnMsgInner{
			Amount: amount,
		},
	}
	jsonMsg, err := json.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	return []byte(spacePad(MessageBlockSize, string(jsonMsg))), nil
}
