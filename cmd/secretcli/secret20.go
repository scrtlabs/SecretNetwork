package main

import (
	"fmt"
	"errors"
	"github.com/enigmampc/SecretNetwork/x/compute/client/cli"
	"github.com/enigmampc/cosmos-sdk/codec"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	"github.com/spf13/cobra"
)

const flagReset = "from"
const flagCoin = "coin"

func S20BalanceCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance [contract address] [account] [viewing_key]",
		Short: "balance",
		Long: `balance`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

			addr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			key := args[1]
			if key == "" {
				return errors.New("viewing key must not be empty")
			}

			queryData := balanceMsg(addr, key)

			err = cli.QueryWithData(cmd, args, cdc, queryData)
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().String(flagReset, "", "")
	cmd.Flags().String(flagCoin, "", "")
	return cmd
}

func S20SendCmd(cdc *codec.Codec) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "secret20 send [contract address] [to account] [amount]",
		Short: "send",
		Long: `send`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			contractAddr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			toAddr, err := sdk.AccAddressFromBech32(args[1])
			if err != nil {
				return err
			}
			amount := args[2]

			msg := sendCoinMsg(toAddr, amount)

			return cli.ExecuteWithData(cmd, contractAddr, msg, amount, false, "", "", cdc)
		},
	}
	cmd.Flags().Bool(flagReset, false, "Optional flag to regenerate the enclave registration key")

	return cmd
}

func balanceMsg(fromAddress sdk.AccAddress, viewingKey string) []byte {
	return []byte(fmt.Sprintf("{\"balance\": {\"address\": \"%s\", \"key\": \"%s\"}}", fromAddress.String(), viewingKey))
}

func sendCoinMsg(toAddress sdk.AccAddress, amount string) []byte {
	return []byte(fmt.Sprintf("{\"transfer\": {\"recipient\": \"%s\", \"amount\": \"%s\"}}", toAddress.String(), amount))
}