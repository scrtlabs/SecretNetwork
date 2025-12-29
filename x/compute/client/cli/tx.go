package cli

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/tx"

	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	wasmUtils "github.com/scrtlabs/SecretNetwork/x/compute/client/utils"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

const (
	flagAmount                 = "amount"
	flagSource                 = "source"
	flagBuilder                = "builder"
	flagLabel                  = "label"
	flagRunAs                  = "run-as"
	flagInstantiateByEverybody = "instantiate-everybody"
	flagInstantiateByAddress   = "instantiate-only-address"
	flagProposalType           = "type"
	flagIoMasterKey            = "enclave-key"
	flagCodeHash               = "code-hash"
	flagAdmin                  = "admin"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Aliases:                    []string{"wasm"},
		Short:                      "Compute transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	txCmd.AddCommand(
		StoreCodeCmd(),
		InstantiateContractCmd(),
		ExecuteContractCmd(),
		MigrateContractCmd(),
		UpdateContractAdminCmd(),
		ClearContractAdminCmd(),
		UpgradeProposalPassedCmd(),
		UpdateMachineWhitelistCmd(),
		SetContractGovernanceCmd(),
	)
	return txCmd
}

// StoreCodeCmd will upload code to be reused.
func StoreCodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store [wasm file] --source [source] --builder [builder]",
		Short: "Upload a WASM binary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg, err := parseStoreCodeArgs(args, clientCtx, cmd.Flags())
			if err != nil {
				return err
			}
			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	cmd.Flags().String(flagSource, "", "A valid URI reference to the contract's source code, optional")
	cmd.Flags().String(flagBuilder, "", "A valid docker tag for the build system, optional")
	cmd.Flags().String(flagInstantiateByEverybody, "", "Everybody can instantiate a contract from the code, optional")
	cmd.Flags().String(flagInstantiateByAddress, "", "Only this address can instantiate a contract instance from the code, optional")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func parseStoreCodeArgs(args []string, cliCtx client.Context, flags *flag.FlagSet) (types.MsgStoreCode, error) {
	wasm, err := os.ReadFile(args[0])
	if err != nil {
		return types.MsgStoreCode{}, err
	}

	// gzip the wasm file
	if wasmUtils.IsWasm(wasm) {
		wasm, err = wasmUtils.GzipIt(wasm)
		if err != nil {
			return types.MsgStoreCode{}, err
		}
	} else if !wasmUtils.IsGzip(wasm) {
		return types.MsgStoreCode{}, fmt.Errorf("invalid input file. Use wasm binary or gzip")
	}

	source, err := flags.GetString(flagSource)
	if err != nil {
		return types.MsgStoreCode{}, fmt.Errorf("source: %s", err)
	}
	builder, err := flags.GetString(flagBuilder)
	if err != nil {
		return types.MsgStoreCode{}, fmt.Errorf("builder: %s", err)
	}

	// build and sign the transaction, then broadcast to Tendermint
	msg := types.MsgStoreCode{
		Sender:       cliCtx.GetFromAddress(),
		WASMByteCode: wasm,
		Source:       source,
		Builder:      builder,
	}
	return msg, nil
}

// InstantiateContractCmd will instantiate a contract from previously uploaded code.
func InstantiateContractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "instantiate [code_id_int64] [json_encoded_init_args] --label [text] --amount [coins,optional] --admin [admin_addr_bech32,optional]",
		Short:   "Instantiate a wasm contract",
		Aliases: []string{"init"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg, err := parseInstantiateArgs(args, cliCtx, cmd.Flags())
			if err != nil {
				return err
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msg)
		},
	}

	cmd.Flags().String(flagCodeHash, "", "For offline transactions, use this to specify the target contract's code hash")
	cmd.Flags().String(flagIoMasterKey, "", "For offline transactions, use this to specify the path to the "+
		"io-master-key.txt file, which you can get using the command `secretcli q register secret-network-params` ")
	cmd.Flags().String(flagAmount, "", "Coins to send to the contract during instantiation")
	cmd.Flags().String(flagLabel, "", "A human-readable name for this contract in lists")
	cmd.Flags().String(flagAdmin, "", "Optional: Bech32 address of the admin of the contract")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func parseInstantiateArgs(args []string, cliCtx client.Context, initFlags *flag.FlagSet) (types.MsgInstantiateContract, error) {
	// get the id of the code to instantiate
	codeID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return types.MsgInstantiateContract{}, err
	}

	amountStr, err := initFlags.GetString(flagAmount)
	if err != nil {
		return types.MsgInstantiateContract{}, fmt.Errorf("amount: %s", err)
	}

	amount, err := sdk.ParseCoinsNormalized(amountStr)
	if err != nil {
		return types.MsgInstantiateContract{}, err
	}

	label, err := initFlags.GetString(flagLabel)
	if label == "" {
		return types.MsgInstantiateContract{}, fmt.Errorf("label is required on all contracts")
	}
	if err != nil {
		return types.MsgInstantiateContract{}, err
	}

	wasmCtx := wasmUtils.WASMContext{CLIContext: cliCtx}
	initMsg := types.SecretMsg{}

	var encryptedMsg []byte
	genOnly, err := initFlags.GetBool(flags.FlagGenerateOnly)
	if err != nil && genOnly {
		// if we're creating an offline transaction we just need the path to the io master key
		ioKeyPath, err := initFlags.GetString(flagIoMasterKey)
		if err != nil {
			return types.MsgInstantiateContract{}, fmt.Errorf("ioKeyPath: %s", err)
		}
		if ioKeyPath == "" {
			return types.MsgInstantiateContract{}, fmt.Errorf("missing flag --%s. To create an offline transaction, you must specify path to the enclave key", flagIoMasterKey)
		}

		codeHash, err := initFlags.GetString(flagCodeHash)
		if err != nil {
			return types.MsgInstantiateContract{}, fmt.Errorf("codeHash: %s", err)
		}
		if codeHash == "" {
			return types.MsgInstantiateContract{}, fmt.Errorf("missing flag --%s. To create an offline transaction, you must set the target contract's code hash", flagCodeHash)
		}
		initMsg.CodeHash = []byte(codeHash)
		initMsg.Msg = []byte(args[1])

		encryptedMsg, err = wasmCtx.OfflineEncrypt(initMsg.Serialize(), ioKeyPath)
		if err != nil {
			return types.MsgInstantiateContract{}, fmt.Errorf("ioKeyPath: %s", err)
		}
	} else {
		// if we aren't creating an offline transaction we can validate the chosen label

		res, _ := GetContractAddressByLabel(label, cliCtx)
		if res != "" {
			return types.MsgInstantiateContract{}, fmt.Errorf("label already exists. You must choose a unique label for your contract instance")
		}

		initMsg.CodeHash, err = GetCodeHashByCodeId(cliCtx, args[0])
		if err != nil {
			return types.MsgInstantiateContract{}, err
		}

		// todo: Add check that this is valid json and stuff
		initMsg.Msg = []byte(args[1])

		encryptedMsg, err = wasmCtx.Encrypt(initMsg.Serialize())
	}

	if err != nil {
		return types.MsgInstantiateContract{}, err
	}

	admin, err := initFlags.GetString(flagAdmin)
	if err != nil {
		return types.MsgInstantiateContract{}, fmt.Errorf("admin: %s", err)
	}
	sndr := cliCtx.GetFromAddress()
	// build and sign the transaction, then broadcast to Tendermint
	msg := types.MsgInstantiateContract{
		Sender:           sndr,
		CallbackCodeHash: "",
		CodeID:           codeID,
		Label:            label,
		InitFunds:        amount,
		InitMsg:          encryptedMsg,
	}

	if admin != "" {
		_, err = sdk.AccAddressFromBech32(admin)
		if err != nil {
			return types.MsgInstantiateContract{}, fmt.Errorf("admin address is not in bech32 format: %s", err)
		}

		msg.Admin = admin
	}

	return msg, nil
}

// ExecuteContractCmd will instantiate a contract from previously uploaded code.
func ExecuteContractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "execute [optional: contract_addr_bech32] [json_encoded_send_args]",
		Short:   "Execute a command on a wasm contract",
		Aliases: []string{"exec"},
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			var contractAddr []byte
			var msg []byte
			var codeHash string
			var ioKeyPath string

			genOnly, err := cmd.Flags().GetBool(flags.FlagGenerateOnly)
			if err != nil {
				return err
			}

			amountStr, err := cmd.Flags().GetString(flagAmount)
			if err != nil {
				return err
			}

			if len(args) == 1 {
				if genOnly {
					return fmt.Errorf("offline transactions must contain contract address")
				}

				label, err := cmd.Flags().GetString(flagLabel)
				if err != nil {
					return fmt.Errorf("error with label: %s", err)
				}
				if label == "" {
					return fmt.Errorf("label or bech32 contract address is required")
				}

				cliCtx, err := client.GetClientQueryContext(cmd)
				if err != nil {
					return err
				}

				queryClient := types.NewQueryClient(cliCtx)
				res, err := queryClient.AddressByLabel(
					context.Background(),
					&types.QueryByLabelRequest{
						Label: label,
					},
				)
				if err != nil {
					return sdkerrors.ErrNotFound.Wrapf("Contract by label %s not found. Error:%s", label, err)
				}

				contractAddr, err = sdk.AccAddressFromBech32(res.ContractAddress)
				if err != nil {
					return err
				}
				msg = []byte(args[0])
			} else {
				// get the id of the code to instantiate
				res, err := sdk.AccAddressFromBech32(args[0])
				if err != nil {
					return err
				}

				contractAddr = res
				msg = []byte(args[1])
			}

			if genOnly {

				ioKeyPath, err = cmd.Flags().GetString(flagIoMasterKey)
				if err != nil {
					return fmt.Errorf("error with ioKeyPath: %s", err)
				}
				if ioKeyPath == "" {
					return fmt.Errorf("missing flag --%s. To create an offline transaction, you must specify path to the enclave key", flagIoMasterKey)
				}

				codeHash, err = cmd.Flags().GetString(flagCodeHash)
				if err != nil {
					return fmt.Errorf("error with codeHash: %s", err)
				}
				if codeHash == "" {
					return fmt.Errorf("missing flag --%s. To create an offline transaction, you must set the target contract's code hash", flagCodeHash)
				}
			}

			return ExecuteWithData(cmd, contractAddr, msg, amountStr, genOnly, ioKeyPath, codeHash, cliCtx)
		},
	}

	cmd.Flags().String(flagCodeHash, "", "For offline transactions, use this to specify the target contract's code hash")
	cmd.Flags().String(flagIoMasterKey, "", "For offline transactions, use this to specify the path to the "+
		"io-master-key.txt file, which you can get using the command `secretcli q register secret-network-params` ")
	cmd.Flags().String(flagAmount, "", "Coins to send to the contract along with command")
	cmd.Flags().String(flagLabel, "", "A human-readable name for this contract in lists")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func ExecuteWithData(cmd *cobra.Command, contractAddress sdk.AccAddress, msg []byte, amount string, genOnly bool, ioMasterKeyPath string, codeHash string, cliCtx client.Context) error {
	wasmCtx := wasmUtils.WASMContext{CLIContext: cliCtx}
	execMsg := types.SecretMsg{}

	execMsg.Msg = msg

	coins, err := sdk.ParseCoinsNormalized(amount)
	if err != nil {
		return err
	}

	var encryptedMsg []byte
	if genOnly {
		execMsg.CodeHash = []byte(codeHash)
		encryptedMsg, err = wasmCtx.OfflineEncrypt(execMsg.Serialize(), ioMasterKeyPath)
	} else {
		cliCtx, err := client.GetClientQueryContext(cmd)
		if err != nil {
			return err
		}

		execMsg.CodeHash, err = GetCodeHashByContractAddr(cliCtx, contractAddress.String())
		if err != nil {
			return sdkerrors.ErrNotFound.Wrapf("Contract address %s not found. Error:%s", contractAddress.String(), err)
		}
		encryptedMsg, err = wasmCtx.Encrypt(execMsg.Serialize())
		if err != nil {
			return fmt.Errorf("failed to encrypt the message. Error:%s", err.Error())
		}
	}
	if err != nil {
		return err
	}

	// build and sign the transaction, then broadcast to Tendermint
	msgExec := types.MsgExecuteContract{
		Sender:           cliCtx.GetFromAddress(),
		Contract:         contractAddress,
		CallbackCodeHash: "",
		SentFunds:        coins,
		Msg:              encryptedMsg,
	}
	return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msgExec)
}

func GetCodeHashByCodeId(cliCtx client.Context, codeID string) ([]byte, error) {
	id, err := strconv.Atoi(codeID)
	if err != nil {
		return nil, err
	}
	queryClient := types.NewQueryClient(cliCtx)
	res, err := queryClient.Code(
		context.Background(),
		&types.QueryByCodeIdRequest{
			CodeId: uint64(id),
		},
	)
	if err != nil {
		return nil, err
	}
	// When querying for an unknown code id the output is an empty result (without any error)
	if res == nil {
		return nil, fmt.Errorf("failed to query contract code hash, unknown code id (%s)", codeID)
	}

	return []byte(res.CodeHash), nil
}

func GetCodeHashByContractAddr(cliCtx client.Context, contractAddr string) ([]byte, error) {
	queryClient := types.NewQueryClient(cliCtx)
	res, err := queryClient.CodeHashByContractAddress(
		context.Background(),
		&types.QueryByContractAddressRequest{
			ContractAddress: contractAddr,
		},
	)
	if err != nil {
		return nil, err
	}
	return []byte(res.CodeHash), nil
}

// MigrateContractCmd will migrate a contract to a new code version
func MigrateContractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "migrate [contract_addr_bech32] [new_code_id_int64] [json_encoded_migration_args]",
		Short:   "Migrate a wasm contract to a new code version",
		Aliases: []string{"update", "mig", "m"},
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg, err := parseMigrateContractArgs(args, cliCtx)
			if err != nil {
				return err
			}
			if err := msg.ValidateBasic(); err != nil {
				return nil
			}
			return tx.GenerateOrBroadcastTxCLI(cliCtx, cmd.Flags(), &msg)
		},
		SilenceUsage: true,
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func parseMigrateContractArgs(args []string, cliCtx client.Context) (types.MsgMigrateContract, error) {
	// get the id of the code to instantiate
	codeID, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return types.MsgMigrateContract{}, errorsmod.Wrap(err, "code id")
	}
	migrateMsg := types.SecretMsg{}

	migrateMsg.CodeHash, err = GetCodeHashByCodeId(cliCtx, args[1])
	if err != nil {
		return types.MsgMigrateContract{}, errorsmod.Wrap(err, "code hash")
	}

	migrateMsg.Msg = []byte(args[2])
	wasmCtx := wasmUtils.WASMContext{CLIContext: cliCtx}
	encryptedMsg, err := wasmCtx.Encrypt(migrateMsg.Serialize())
	if err != nil {
		return types.MsgMigrateContract{}, errorsmod.Wrap(err, "encrypt")
	}
	msg := types.MsgMigrateContract{
		Sender:   cliCtx.GetFromAddress().String(),
		Contract: args[0],
		CodeID:   codeID,
		Msg:      encryptedMsg,
	}
	return msg, nil
}

// UpdateContractAdminCmd sets an new admin for a contract
func UpdateContractAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "set-contract-admin [contract_addr_bech32] [new_admin_addr_bech32]",
		Short:   "Set new admin for a contract",
		Aliases: []string{"new-admin", "admin", "set-adm", "sa"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg, err := parseUpdateContractAdminArgs(args, clientCtx)
			if err != nil {
				return err
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
		SilenceUsage: true,
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func parseUpdateContractAdminArgs(args []string, cliCtx client.Context) (types.MsgUpdateAdmin, error) {
	msg := types.MsgUpdateAdmin{
		Sender:   cliCtx.GetFromAddress().String(),
		Contract: args[0],
		NewAdmin: args[1],
	}
	return msg, nil
}

// ClearContractAdminCmd clears an admin for a contract
func ClearContractAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clear-contract-admin [contract_addr_bech32]",
		Short:   "Clears admin for a contract to prevent further migrations",
		Aliases: []string{"clear-admin", "clr-adm"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.MsgClearAdmin{
				Sender:   clientCtx.GetFromAddress().String(),
				Contract: args[0],
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
		SilenceUsage: true,
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// UpgradeProposalPassedCmd
func UpgradeProposalPassedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade-proposal-passed [mrenclave-hash]",
		Short: "Upgrade MREnclave with a new SHA256 hash",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			mrEnclaveHash, err := hex.DecodeString(args[0])
			if err != nil {
				return err
			}

			// Create the message for upgrading the MREnclave
			msg := types.MsgUpgradeProposalPassed{
				SenderAddress: clientCtx.GetFromAddress().String(),
				MrEnclaveHash: mrEnclaveHash,
			}

			// Validate the message
			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			// Generate or broadcast the transaction
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func SetContractGovernanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-contract-governance [contract-address]",
		Short: "Set governance requirement for contract upgrades (admin only)",
		Long: `Set whether a contract requires governance approval for upgrades. 
Only the contract admin can execute this command.

Note: This is a one-way operation. Once governance is required (true), 
it cannot be changed back to false.

Examples:
  # Enable governance requirement
  secretd tx compute set-contract-governance secret1abc123... --from contract-admin`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			contractAddr := args[0]
			if err != nil {
				return fmt.Errorf("invalid require-governance value '%s', must be true or false", args[1])
			}

			msg := &types.MsgSetContractGovernance{
				Sender:          clientCtx.GetFromAddress().String(),
				ContractAddress: contractAddr,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func ParseHexList(s string) ([][]byte, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil // or empty slice, your choice
	}

	parts := strings.Split(s, ",")
	out := make([][]byte, 0, len(parts))

	for i, p := range parts {
		p = strings.TrimSpace(p)

		b, err := hex.DecodeString(p)
		if err != nil {
			return nil, fmt.Errorf("invalid hex token #%d (%q): %w", i, p, err)
		}

		out = append(out, b)
	}

	return out, nil
}

func UpdateMachineWhitelistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-machine-whitelist [proposal-id] [machine-id]",
		Short: "Update machine whitelist after governance approval",
		Long: `Execute machine whitelist update after governance proposal passes.
Machine ID must match the approved proposal exactly.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			proposalID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid proposal ID: %w", err)
			}

			// Read machine ID
			machineId := args[1]

			ids, err := ParseHexList(machineId)
			if err != nil {
				return fmt.Errorf("machine_id malformed")
			}
			if len(ids) == 0 {
				return fmt.Errorf("machine_id must not be empty")
			}

			msg := &types.MsgUpdateMachineWhitelist{
				Sender:     clientCtx.GetFromAddress().String(),
				ProposalId: proposalID,
				MachineId:  machineId,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
