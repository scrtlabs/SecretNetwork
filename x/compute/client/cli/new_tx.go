package cli

import (
	"bufio"
	"strconv"

	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
	"github.com/enigmampc/cosmos-sdk/client/context"
	"github.com/enigmampc/cosmos-sdk/codec"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	sdkerrors "github.com/enigmampc/cosmos-sdk/types/errors"
	"github.com/enigmampc/cosmos-sdk/x/auth"
	"github.com/enigmampc/cosmos-sdk/x/auth/client/utils"
	"github.com/spf13/cobra"
)

// MigrateContractCmd will migrate a contract to a new code version
// NO SUPPORT FOR THIS COMMAND YET
func MigrateContractCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate [contract_addr_bech32] [new_code_id_int64] [json_encoded_migration_args]",
		Short: "Migrate a wasm contract to a new code version",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := auth.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			msg, err := parseMigrateContractArgs(args, cliCtx)
			if err != nil {
				return err
			}
			if err := msg.ValidateBasic(); err != nil {
				return nil
			}
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}
	return cmd
}

func parseMigrateContractArgs(args []string, cliCtx context.CLIContext) (types.MsgMigrateContract, error) {
	contractAddr, err := sdk.AccAddressFromBech32(args[0])
	if err != nil {
		return types.MsgMigrateContract{}, sdkerrors.Wrap(err, "contract")
	}

	// get the id of the code to instantiate
	codeID, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return types.MsgMigrateContract{}, sdkerrors.Wrap(err, "code id")
	}

	migrateMsg := args[2]

	msg := types.MsgMigrateContract{
		Sender:     cliCtx.GetFromAddress(),
		Contract:   contractAddr,
		CodeID:     codeID,
		MigrateMsg: []byte(migrateMsg),
	}
	return msg, nil
}

// UpdateContractAdminCmd sets an new admin for a contract
// NO SUPPORT FOR THIS COMMAND YET
func UpdateContractAdminCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-contract-admin [contract_addr_bech32] [new_admin_addr_bech32]",
		Short: "Set new admin for a contract",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := auth.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			msg, err := parseUpdateContractAdminArgs(args, cliCtx)
			if err != nil {
				return err
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}
	return cmd
}

func parseUpdateContractAdminArgs(args []string, cliCtx context.CLIContext) (types.MsgUpdateAdmin, error) {
	contractAddr, err := sdk.AccAddressFromBech32(args[0])
	if err != nil {
		return types.MsgUpdateAdmin{}, sdkerrors.Wrap(err, "contract")
	}
	newAdmin, err := sdk.AccAddressFromBech32(args[1])
	if err != nil {
		return types.MsgUpdateAdmin{}, sdkerrors.Wrap(err, "new admin")
	}

	msg := types.MsgUpdateAdmin{
		Sender:   cliCtx.GetFromAddress(),
		Contract: contractAddr,
		NewAdmin: newAdmin,
	}
	return msg, nil
}

// ClearContractAdminCmd clears an admin for a contract
// NO SUPPORT FOR THIS COMMAND YET
func ClearContractAdminCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear-contract-admin [contract_addr_bech32]",
		Short: "Clears admin for a contract to prevent further migrations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := auth.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			contractAddr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return sdkerrors.Wrap(err, "contract")
			}

			msg := types.MsgClearAdmin{
				Sender:   cliCtx.GetFromAddress(),
				Contract: contractAddr,
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}
	return cmd
}
