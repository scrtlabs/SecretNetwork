package cli

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/keeper"
	"io/ioutil"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/enigmampc/cosmos-sdk/client"
	"github.com/enigmampc/cosmos-sdk/client/context"
	"github.com/enigmampc/cosmos-sdk/client/flags"
	"github.com/enigmampc/cosmos-sdk/codec"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	sdkerrors "github.com/enigmampc/cosmos-sdk/types/errors"
	"github.com/enigmampc/cosmos-sdk/x/auth"
	"github.com/enigmampc/cosmos-sdk/x/auth/client/utils"

	wasmUtils "github.com/enigmampc/SecretNetwork/x/compute/client/utils"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

const (
	flagAmount                 = "amount"
	flagSource                 = "source"
	flagBuilder                = "builder"
	flagLabel                  = "label"
	flagAdmin                  = "admin"
	flagRunAs                  = "run-as"
	flagInstantiateByEverybody = "instantiate-everybody"
	flagInstantiateByAddress   = "instantiate-only-address"
	flagProposalType           = "type"
	flagIoMasterKey = "enclave-key"
	flagCodeHash    = "code-hash"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd(cdc *codec.Codec) *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Compute transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	txCmd.AddCommand(flags.PostCommands(
		StoreCodeCmd(cdc),
		InstantiateContractCmd(cdc),
		ExecuteContractCmd(cdc),
		// Currently not supporting these commands
		//MigrateContractCmd(cdc),
		//UpdateContractAdminCmd(cdc),
		//ClearContractAdminCmd(cdc),
	)...)
	return txCmd
}

// StoreCodeCmd will upload code to be reused.
func StoreCodeCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store [wasm file] --source [source] --builder [builder]",
		Short: "Upload a wasm binary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := auth.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)

			msg, err := parseStoreCodeArgs(args, cliCtx)
			if err != nil {
				return err
			}
			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagSource, "", "A valid URI reference to the contract's source code, optional")
	cmd.Flags().String(flagBuilder, "", "A valid docker tag for the build system, optional")
	cmd.Flags().String(flagInstantiateByEverybody, "", "Everybody can instantiate a contract from the code, optional")
	cmd.Flags().String(flagInstantiateByAddress, "", "Only this address can instantiate a contract instance from the code, optional")

	return cmd
}

func parseStoreCodeArgs(args []string, cliCtx context.CLIContext) (types.MsgStoreCode, error) {
	wasm, err := ioutil.ReadFile(args[0])
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

	var perm *types.AccessConfig
	if onlyAddrStr := viper.GetString(flagInstantiateByAddress); onlyAddrStr != "" {
		allowedAddr, err := sdk.AccAddressFromBech32(onlyAddrStr)
		if err != nil {
			return types.MsgStoreCode{}, sdkerrors.Wrap(err, flagInstantiateByAddress)
		}
		x := types.OnlyAddress.With(allowedAddr)
		perm = &x
	} else if everybody := viper.GetBool(flagInstantiateByEverybody); everybody {
		perm = &types.AllowEverybody
	}

	// build and sign the transaction, then broadcast to Tendermint
	msg := types.MsgStoreCode{
		Sender:                cliCtx.GetFromAddress(),
		WASMByteCode:          wasm,
		Source:                viper.GetString(flagSource),
		Builder:               viper.GetString(flagBuilder),
		InstantiatePermission: perm,
	}
	return msg, nil
}

// InstantiateContractCmd will instantiate a contract from previously uploaded code.
func InstantiateContractCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instantiate [code_id_int64] [json_encoded_init_args] --label [text] --admin [address,optional] --amount [coins,optional]",
		Short: "Instantiate a wasm contract",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := auth.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)
			wasmCtx := wasmUtils.WASMContext{CLIContext: cliCtx}
			initMsg := types.SecretMsg{}

			msg, err := parseInstantiateArgs(args, cliCtx)
			if err != nil {
				return err
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagCodeHash, "", "For offline transactions, use this to specify the target contract's code hash")
	cmd.Flags().String(flagIoMasterKey, "", "For offline transactions, use this to specify the path to the "+
		"io-master-cert.der file, which you can get using the command `secretcli q register secret-network-params` ")
	cmd.Flags().String(flagAmount, "", "Coins to send to the contract during instantiation")
	cmd.Flags().String(flagLabel, "", "A human-readable name for this contract in lists")
	cmd.Flags().String(flagAdmin, "", "Address of an admin")
	return cmd
}

func parseInstantiateArgs(args []string, cliCtx context.CLIContext) (types.MsgInstantiateContract, error) {
	// get the id of the code to instantiate
	codeID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return types.MsgInstantiateContract{}, err
	}

	amounstStr := viper.GetString(flagAmount)
	amount, err := sdk.ParseCoins(amounstStr)
	if err != nil {
		return types.MsgInstantiateContract{}, err
	}

	label := viper.GetString(flagLabel)
	if label == "" {
		return types.MsgInstantiateContract{}, fmt.Errorf("Label is required on all contracts")
	}

	var encryptedMsg []byte
	if viper.GetBool(flags.FlagGenerateOnly) {
		// if we're creating an offline transaction we just need the path to the io master key
		ioKeyPath := viper.GetString(flagIoMasterKey)

		if ioKeyPath == "" {
			return types.MsgInstantiateContract{}, fmt.Errorf("missing flag --%s. To create an offline transaction, you must specify path to the enclave key", flagIoMasterKey)
		}

		codeHash := viper.GetString(flagCodeHash)
		if codeHash == "" {
			return types.MsgInstantiateContract{}, fmt.Errorf("missing flag --%s. To create an offline transaction, you must set the target contract's code hash", flagCodeHash)
		}
		initMsg.CodeHash = []byte(codeHash)

		encryptedMsg, err = wasmCtx.OfflineEncrypt(initMsg.Serialize(), ioKeyPath)
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

	adminStr := viper.GetString(flagAdmin)
	var adminAddr sdk.AccAddress
	if len(adminStr) != 0 {
		adminAddr, err = sdk.AccAddressFromBech32(adminStr)
		if err != nil {
			return types.MsgInstantiateContract{}, sdkerrors.Wrap(err, "admin")
		}
	}

	// build and sign the transaction, then broadcast to Tendermint
	msg := types.MsgInstantiateContract{
		Sender:           cliCtx.GetFromAddress(),
		CallbackCodeHash: "",
		CodeID:           codeID,
		Label:            label,
		InitFunds:        amount,
		InitMsg:          encryptedMsg,
		Admin:            adminAddr,
	}
	return msg, nil
}

// ExecuteContractCmd will instantiate a contract from previously uploaded code.
func ExecuteContractCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execute [optional: contract_addr_bech32] [json_encoded_send_args]",
		Short: "Execute a command on a wasm contract",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inBuf := bufio.NewReader(cmd.InOrStdin())
			txBldr := auth.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))
			cliCtx := context.NewCLIContextWithInput(inBuf).WithCodec(cdc)
			wasmCtx := wasmUtils.WASMContext{CLIContext: cliCtx}
			execMsg := types.SecretMsg{}
			contractAddr := sdk.AccAddress{}

			if len(args) == 1 {

				if viper.GetBool(flags.FlagGenerateOnly) {
					return fmt.Errorf("offline transactions must contain contract address")
				}

				label := viper.GetString(flagLabel)
				if label == "" {
					return fmt.Errorf("label or bech32 contract address is required")
				}

				route := fmt.Sprintf("custom/%s/%s/%s", types.QuerierRoute, keeper.QueryContractAddress, label)
				res, _, err := cliCtx.Query(route)
				if err != nil {
					return err
				}

				contractAddr = res
				execMsg.Msg = []byte(args[0])
			} else {
				// get the id of the code to instantiate
				res, err := sdk.AccAddressFromBech32(args[0])
				if err != nil {
					return err
				}

				contractAddr = res
				execMsg.Msg = []byte(args[1])
			}

			amounstStr := viper.GetString(flagAmount)
			amount, err := sdk.ParseCoins(amounstStr)
			if err != nil {
				return err
			}

			var encryptedMsg []byte
			if viper.GetBool(flags.FlagGenerateOnly) {
				ioKeyPath := viper.GetString(flagIoMasterKey)

				if ioKeyPath == "" {
					return fmt.Errorf("missing flag --%s. To create an offline transaction, you must specify path to the enclave key", flagIoMasterKey)
				}

				codeHash := viper.GetString(flagCodeHash)
				if codeHash == "" {
					return fmt.Errorf("missing flag --%s. To create an offline transaction, you must set the target contract's code hash", flagCodeHash)
				}
				execMsg.CodeHash = []byte(codeHash)

				encryptedMsg, err = wasmCtx.OfflineEncrypt(execMsg.Serialize(), ioKeyPath)
			} else {
				execMsg.CodeHash, err = GetCodeHashByContractAddr(cliCtx, contractAddr)
				if err != nil {
					return err
				}

				encryptedMsg, err = wasmCtx.Encrypt(execMsg.Serialize())
			}
			if err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			msg := types.MsgExecuteContract{
				Sender:           cliCtx.GetFromAddress(),
				Contract:         contractAddr,
				CallbackCodeHash: "",
				SentFunds:        amount,
				Msg:              encryptedMsg,
			}
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagCodeHash, "", "For offline transactions, use this to specify the target contract's code hash")
	cmd.Flags().String(flagIoMasterKey, "", "For offline transactions, use this to specify the path to the "+
		"io-master-cert.der file, which you can get using the command `secretcli q register secret-network-params` ")
	cmd.Flags().String(flagAmount, "", "Coins to send to the contract along with command")
	cmd.Flags().String(flagLabel, "", "A human-readable name for this contract in lists")
	return cmd
}

func GetCodeHashByCodeId(cliCtx context.CLIContext, codeID string) ([]byte, error) {
	route := fmt.Sprintf("custom/%s/%s/%s", types.QuerierRoute, keeper.QueryGetCode, codeID)
	res, _, err := cliCtx.Query(route)
	if err != nil {
		return nil, err
	}

	var codeResp keeper.GetCodeResponse

	err = json.Unmarshal(res, &codeResp)
	if err != nil {
		return nil, err
	}

	return []byte(hex.EncodeToString(codeResp.DataHash)), nil
}

func GetCodeHashByContractAddr(cliCtx context.CLIContext, contractAddr sdk.AccAddress) ([]byte, error) {
	route := fmt.Sprintf("custom/%s/%s/%s", types.QuerierRoute, keeper.QueryContractHash, contractAddr.String())
	res, _, err := cliCtx.Query(route)
	if err != nil {
		return nil, err
	}

	return []byte(hex.EncodeToString(res)), nil
}
