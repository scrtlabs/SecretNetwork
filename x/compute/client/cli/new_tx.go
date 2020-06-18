package cli

import (
	"bufio"
	"errors"
	"strconv"

	"github.com/CosmWasm/wasmd/x/wasm/internal/types"
	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/client/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// MigrateContractCmd will migrate a contract to a new code version
func MigrateContractCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate [contract_addr_bech32] [new_code_id_int64] [json_encoded_migration_args]",
		Short: "Migrate a wasm contract to a new code version",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := auth.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			contractAddr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return sdkerrors.Wrap(err, "contract")
			}

			// get the id of the code to instantiate
			codeID, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return sdkerrors.Wrap(err, "code id")
			}

			migrateMsg := args[2]

			msg := types.MsgMigrateContract{
				Sender:     cliCtx.GetFromAddress(),
				Contract:   contractAddr,
				Code:       codeID,
				MigrateMsg: []byte(migrateMsg),
			}
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}
	return cmd
}

// UpdateContractAdminCmd sets or clears an admin for a contract
func UpdateContractAdminCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-contract-admin [contract_addr_bech32] [new_admin_addr_bech32]",
		Short: "Set new admin for a contract. Can be empty to prevent further migrations",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := auth.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			contractAddr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return sdkerrors.Wrap(err, "contract")
			}
			var newAdmin sdk.AccAddress
			if len(args) > 1 && len(args[1]) != 0 {
				newAdmin, err = sdk.AccAddressFromBech32(args[1])
				if err != nil {
					return sdkerrors.Wrap(err, "new admin")
				}
			} else {
				// safety net to not accidentally clear an admin
				clearAdmin := viper.GetBool(flagNoAdmin)
				if !clearAdmin {
					return errors.New("new admin address required or no admin flag")
				}
			}

			msg := types.MsgUpdateAdministrator{
				Sender:   cliCtx.GetFromAddress(),
				Contract: contractAddr,
				NewAdmin: newAdmin,
			}
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}
	cmd.Flags().Bool(flagNoAdmin, false, "Remove admin which disables future admin updates and migrations")
	return cmd
}
