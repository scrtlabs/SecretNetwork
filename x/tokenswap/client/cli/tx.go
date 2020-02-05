package cli

import (
	"bufio"
	"strconv"

	"github.com/enigmampc/Enigmachain/x/tokenswap/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/x/auth/client/utils"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// GetCmdCreateEthBridgeClaim is the CLI command for creating a claim on an ethereum prophecy
func GetCmdCreateEthBridgeClaim(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "create-claim [ethereum-chain-id] [bridge-contract] [nonce] [symbol] [token-contract] [ethereum-sender-address] [cosmos-receiver-address] [validator-address] [amount] [claim-type]",
		Short: "create a claim on an ethereum prophecy",
		Args:  cobra.ExactArgs(10),
		RunE: func(cmd *cobra.Command, args []string) error {

			cliCtx := context.NewCLIContext().WithCodec(cdc)
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := authtypes.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))

			ethereumChainID, err := strconv.Atoi(args[0])
			if err != nil {
				return err
			}

			bridgeContract := types.NewEthereumAddress(args[1])

			nonce, err := strconv.Atoi(args[2])
			if err != nil {
				return err
			}

			symbol := args[3]
			tokenContract := types.NewEthereumAddress(args[4])
			ethereumSender := types.NewEthereumAddress(args[5])
			cosmosReceiver, err := sdk.AccAddressFromBech32(args[6])
			if err != nil {
				return err
			}

			validator, err := sdk.ValAddressFromBech32(args[7])
			if err != nil {
				return err
			}

			amount, err := sdk.ParseCoins(args[8])
			if err != nil {
				return err
			}

			claimType, err := types.StringToClaimType(args[9])
			if err != nil {
				return err
			}

			ethBridgeClaim := types.NewEthBridgeClaim(ethereumChainID, bridgeContract, nonce, symbol, tokenContract, ethereumSender, cosmosReceiver, validator, amount, claimType)

			msg := types.NewMsgCreateEthBridgeClaim(ethBridgeClaim)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}
}

// GetCmdBurn is the CLI command for burning some of your eth and triggering an event
func GetCmdBurn(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "burn [cosmos-sender-address] [ethereum-receiver-address] [amount] --ethereum-chain-id [ethereum-chain-id] --token-contract-address [token-contract-address]",
		Short: "burn cETH or cERC20 on the Cosmos chain",
		Long:  "This should be used to burn cETH or cERC20. It will burn your coins on the Cosmos Chain, removing them from your account and deducting them from the supply. It will also trigger an event on the Cosmos Chain for relayers to watch so that they can trigger the withdrawal of the original ETH/ERC20 to you from the Ethereum contract!",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			cliCtx := context.NewCLIContext().WithCodec(cdc)
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := authtypes.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))

			ethereumChainIDString := viper.GetString(types.FlagEthereumChainID)
			ethereumChainID, err := strconv.Atoi(ethereumChainIDString)
			if err != nil {
				return err
			}

			tokenContractString := viper.GetString(types.FlagTokenContractAddr)
			tokenContract := types.NewEthereumAddress(tokenContractString)

			cosmosSender, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			ethereumReceiver := types.NewEthereumAddress(args[1])
			amount, err := sdk.ParseCoins(args[2])
			if err != nil {
				return err
			}

			msg := types.NewMsgBurn(ethereumChainID, tokenContract, cosmosSender, ethereumReceiver, amount)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}
}

// GetCmdLock is the CLI command for locking some of your coins and triggering an event
func GetCmdLock(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "lock [cosmos-sender-address] [ethereum-receiver-address] [amount] --ethereum-chain-id [ethereum-chain-id] --token-contract-address [token-contract-address]",
		Short: "This should be used to lock Cosmos-originating coins (eg: ATOM). It will lock up your coins in the supply module, removing them from your account. It will also trigger an event on the Cosmos Chain for relayers to watch so that they can trigger the minting of the pegged token on Etherum to you!",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			cliCtx := context.NewCLIContext().WithCodec(cdc)
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := authtypes.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))

			ethereumChainIDString := viper.GetString(types.FlagEthereumChainID)
			ethereumChainID, err := strconv.Atoi(ethereumChainIDString)
			if err != nil {
				return err
			}

			tokenContractString := viper.GetString(types.FlagTokenContractAddr)
			tokenContract := types.NewEthereumAddress(tokenContractString)

			cosmosSender, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			ethereumReceiver := types.NewEthereumAddress(args[1])
			amount, err := sdk.ParseCoins(args[2])
			if err != nil {
				return err
			}

			msg := types.NewMsgLock(ethereumChainID, tokenContract, cosmosSender, ethereumReceiver, amount)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}
}
