package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/scrtlabs/SecretNetwork/x/compute/internal/keeper"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
		Use:     "instantiate [code_id_int64] [json_encoded_init_args] --label [text] --amount [coins,optional]",
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
		"io-master-cert.der file, which you can get using the command `secretcli q register secret-network-params` ")
	cmd.Flags().String(flagAmount, "", "Coins to send to the contract during instantiation")
	cmd.Flags().String(flagLabel, "", "A human-readable name for this contract in lists")
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
		route := fmt.Sprintf("custom/%s/%s/%s", types.QuerierRoute, keeper.QueryContractAddress, label)
		res, _, _ := cliCtx.Query(route)
		if res != nil {
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

	// build and sign the transaction, then broadcast to Tendermint
	msg := types.MsgInstantiateContract{
		Sender:           cliCtx.GetFromAddress(),
		CallbackCodeHash: "",
		CodeID:           codeID,
		Label:            label,
		InitFunds:        amount,
		InitMsg:          encryptedMsg,
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

				route := fmt.Sprintf("custom/%s/%s/%s", types.QuerierRoute, keeper.QueryContractAddress, label)
				res, _, err := cliCtx.Query(route)
				if err != nil {
					return err
				}

				contractAddr = res
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
		"io-master-cert.der file, which you can get using the command `secretcli q register secret-network-params` ")
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
		execMsg.CodeHash, err = GetCodeHashByContractAddr(cliCtx, contractAddress)
		if err != nil {
			return err
		}
		encryptedMsg, err = wasmCtx.Encrypt(execMsg.Serialize())
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
	route := fmt.Sprintf("custom/%s/%s/%s", types.QuerierRoute, keeper.QueryGetCode, codeID)
	res, _, err := cliCtx.Query(route)
	if err != nil {
		return nil, err
	}

	// When querying for an unknown code id the output is an empty result (without any error)
	if len(res) == 0 {
		return nil, fmt.Errorf("failed to query contract code hash, unknown code id (%s)", codeID)
	}

	var codeResp types.QueryCodeResponse

	err = json.Unmarshal(res, &codeResp)
	if err != nil {
		return nil, err
	}

	return []byte(codeResp.CodeHash), nil
}

func GetCodeHashByContractAddr(cliCtx client.Context, contractAddr sdk.AccAddress) ([]byte, error) {
	route := fmt.Sprintf("custom/%s/%s/%s", types.QuerierRoute, keeper.QueryContractHash, contractAddr.String())
	res, _, err := cliCtx.Query(route)
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, fmt.Errorf("contract with address %s not found", contractAddr.String())
	}

	return []byte(hex.EncodeToString(res)), nil
}
