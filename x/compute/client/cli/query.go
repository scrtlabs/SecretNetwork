package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	cosmwasmTypes "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	flag "github.com/spf13/pflag"
	"github.com/tendermint/go-amino"

	"github.com/spf13/cobra"

	"github.com/enigmampc/cosmos-sdk/client"
	"github.com/enigmampc/cosmos-sdk/client/context"
	"github.com/enigmampc/cosmos-sdk/client/flags"
	"github.com/enigmampc/cosmos-sdk/codec"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	"github.com/enigmampc/cosmos-sdk/x/auth/client/utils"

	wasmUtils "github.com/enigmampc/SecretNetwork/x/compute/client/utils"

	"github.com/enigmampc/SecretNetwork/x/compute/internal/keeper"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

func GetQueryCmd(cdc *codec.Codec) *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the compute module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	queryCmd.AddCommand(flags.GetCommands(
		GetCmdListCode(cdc),
		GetCmdListContractByCode(cdc),
		GetCmdQueryCode(cdc),
		GetCmdGetContractInfo(cdc),
		GetCmdGetContractState(cdc),
		GetQueryDecryptTxCmd(cdc),
	)...)
	return queryCmd
}

// GetCmdListCode lists all wasm code uploaded
func GetCmdListCode(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "list-code",
		Short: "List all wasm bytecode on the chain",
		Long:  "List all wasm bytecode on the chain",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)

			route := fmt.Sprintf("custom/%s/%s", types.QuerierRoute, keeper.QueryListCode)
			res, _, err := cliCtx.Query(route)
			if err != nil {
				return err
			}
			fmt.Println(string(res))
			return nil
		},
	}
}

// GetCmdListContractByCode lists all wasm code uploaded for given code id
func GetCmdListContractByCode(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "list-contract-by-code [code_id]",
		Short: "List wasm all bytecode on the chain for given code id",
		Long:  "List wasm all bytecode on the chain for given code id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)

			codeID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			route := fmt.Sprintf("custom/%s/%s/%d", types.QuerierRoute, keeper.QueryListContractByCode, codeID)
			res, _, err := cliCtx.Query(route)
			if err != nil {
				return err
			}
			fmt.Println(string(res))
			return nil
		},
	}
}

// GetCmdQueryCode returns the bytecode for a given contract
func GetCmdQueryCode(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "code [code_id] [output filename]",
		Short: "Downloads wasm bytecode for given code id",
		Long:  "Downloads wasm bytecode for given code id",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)

			codeID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			route := fmt.Sprintf("custom/%s/%s/%d", types.QuerierRoute, keeper.QueryGetCode, codeID)
			res, _, err := cliCtx.Query(route)
			if err != nil {
				return err
			}
			if len(res) == 0 {
				return fmt.Errorf("contract not found")
			}
			var code keeper.GetCodeResponse
			err = json.Unmarshal(res, &code)
			if err != nil {
				return err
			}

			if len(code.Data) == 0 {
				return fmt.Errorf("contract not found")
			}

			fmt.Printf("Downloading wasm code to %s\n", args[1])
			return ioutil.WriteFile(args[1], code.Data, 0644)
		},
	}
}

// GetCmdGetContractInfo gets details about a given contract
func GetCmdGetContractInfo(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "contract [bech32_address]",
		Short: "Prints out metadata of a contract given its address",
		Long:  "Prints out metadata of a contract given its address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)

			addr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			route := fmt.Sprintf("custom/%s/%s/%s", types.QuerierRoute, keeper.QueryGetContract, addr.String())
			res, _, err := cliCtx.Query(route)
			if err != nil {
				return err
			}
			fmt.Println(string(res))
			return nil
		},
	}
}

// GetCmdGetContractState dumps full internal state of a given contract
func GetCmdGetContractState(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "contract-state",
		Short:                      "Querying commands for the wasm module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(flags.GetCommands(
		GetCmdGetContractStateAll(cdc),
		GetCmdGetContractStateRaw(cdc),
		GetCmdGetContractStateSmart(cdc),
	)...)
	return cmd

}

func GetCmdGetContractStateAll(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "all [bech32_address]",
		Short: "Prints out all internal state of a contract given its address",
		Long:  "Prints out all internal state of a contract given its address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)

			addr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			route := fmt.Sprintf("custom/%s/%s/%s/%s", types.QuerierRoute, keeper.QueryGetContractState, addr.String(), keeper.QueryMethodContractStateAll)
			res, _, err := cliCtx.Query(route)
			if err != nil {
				return err
			}
			fmt.Println(string(res))
			return nil
		},
	}
}

func GetCmdGetContractStateRaw(cdc *codec.Codec) *cobra.Command {
	decoder := newArgDecoder(hex.DecodeString)
	cmd := &cobra.Command{
		Use:   "raw [bech32_address] [key]",
		Short: "Prints out internal state for key of a contract given its address",
		Long:  "Prints out internal state for of a contract given its address",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)

			addr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}
			queryData, err := decoder.DecodeString(args[1])
			if err != nil {
				return err
			}
			route := fmt.Sprintf("custom/%s/%s/%s/%s", types.QuerierRoute, keeper.QueryGetContractState, addr.String(), keeper.QueryMethodContractStateRaw)
			res, _, err := cliCtx.QueryWithData(route, queryData)
			if err != nil {
				return err
			}
			fmt.Println(string(res))
			return nil
		},
	}
	decoder.RegisterFlags(cmd.PersistentFlags(), "key argument")
	return cmd
}

// QueryDecryptTxCmd the default command for a tx query + IO decryption if I'm the tx sender.
// Coppied from https://github.com/enigmampc/cosmos-sdk/blob/v0.38.4/x/auth/client/cli/query.go#L157-L184 and added IO decryption (Could not wrap it because it prints directly to stdout)
func GetQueryDecryptTxCmd(cdc *amino.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx [hash]",
		Short: "Query for a transaction by hash in a committed block, decrypt input and outputs if I'm the tx sender",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)

			result, err := utils.QueryTx(cliCtx, args[0])
			if err != nil {
				return err
			}

			if result.Empty() {
				return fmt.Errorf("no transaction found with hash %s", args[0])
			}

			var answer struct {
				Type           string                 `json:"type"`
				Input          string                 `json:"input"`
				OutputData     string                 `json:"output_data"`
				OutputLogs     []sdk.StringEvent      `json:"output_log"`
				OutputError    cosmwasmTypes.StdError `json:"output_error"`
				PlaintextError string                 `json:"plaintext_error"`
			}
			var encryptedInput []byte
			var dataOutputHexB64 string

			txInputs := result.Tx.GetMsgs()
			if len(txInputs) != 1 {
				return fmt.Errorf("Can only decrypt txs with 1 input. Got %d", len(txInputs))
			}
			txInput := txInputs[0]

			if txInput.Type() == "execute" {
				execTx, ok := txInput.(*types.MsgExecuteContract)
				if !ok {
					return fmt.Errorf("Error parsing tx as type 'execute': %v", txInput)
				}

				encryptedInput = execTx.Msg
				dataOutputHexB64 = result.Data
			} else if txInput.Type() == "instantiate" {
				initTx, ok := txInput.(*types.MsgInstantiateContract)
				if !ok {
					return fmt.Errorf("Error parsing tx as type 'instantiate': %v", txInput)
				}

				encryptedInput = initTx.InitMsg
			} else {
				return fmt.Errorf("Tx %s is not of type 'execute' or 'instantiate'. Got type '%s'", args[0], txInput.Type())
			}
			answer.Type = txInput.Type()

			// decrypt input
			if len(encryptedInput) < 64 {
				return fmt.Errorf("Input must be > 64 bytes. Got %d", len(encryptedInput))
			}

			nonce := encryptedInput[0:32]
			originalTxSenderPubkey := encryptedInput[32:64]

			wasmCtx := wasmUtils.WASMContext{CLIContext: cliCtx}
			_, myPubkey, err := wasmCtx.GetTxSenderKeyPair()
			if err != nil {
				return err
			}

			if !bytes.Equal(originalTxSenderPubkey, myPubkey) {
				return fmt.Errorf("Cannot decrypt, not original tx sender")
			}

			ciphertextInput := encryptedInput[64:]
			plaintextInput := []byte{}
			if len(ciphertextInput) > 0 {
				plaintextInput, err = wasmCtx.Decrypt(ciphertextInput, nonce)
				if err != nil {
					return err
				}
			}

			answer.Input = string(plaintextInput)

			// decrypt data
			if answer.Type == "execute" {
				dataOutputB64, err := hex.DecodeString(dataOutputHexB64)
				if err != nil {
					return err
				}

				dataOutputCipherBz, err := base64.StdEncoding.DecodeString(string(dataOutputB64))
				if err != nil {
					return err
				}

				dataPlaintext, err := wasmCtx.Decrypt(dataOutputCipherBz, nonce)
				if err != nil {
					return err
				}
				answer.OutputData = string(dataPlaintext)
			}

			// decrypt logs
			answer.OutputLogs = []sdk.StringEvent{}
			for _, l := range result.Logs {
				for _, e := range l.Events {
					if e.Type == "wasm" {
						for i, a := range e.Attributes {
							if a.Key != "contract_address" {
								// key
								if a.Key != "" {
									keyCiphertext, err := base64.StdEncoding.DecodeString(a.Key)
									if err != nil {
										return err
									}
									keyPlaintext, err := wasmCtx.Decrypt(keyCiphertext, nonce)
									if err != nil {
										return err
									}
									a.Key = string(keyPlaintext)
								}

								// value
								if a.Value != "" {
									valueCiphertext, err := base64.StdEncoding.DecodeString(a.Value)
									if err != nil {
										return err
									}
									valuePlaintext, err := wasmCtx.Decrypt(valueCiphertext, nonce)
									if err != nil {
										return err
									}
									a.Value = string(valuePlaintext)
								}

								e.Attributes[i] = a
							}
						}
						answer.OutputLogs = append(answer.OutputLogs, e)
					}
				}
			}

			if strings.Contains(result.RawLog, "wasm contract failed: generic: ") {
				errorCipherB64 := strings.ReplaceAll(result.RawLog, answer.Type+" wasm contract failed: generic: ", "")
				errorCipherB64 = strings.ReplaceAll(errorCipherB64, ": failed to execute message; message index: 0", "")

				errorCipherBz, err := base64.StdEncoding.DecodeString(errorCipherB64)
				if err != nil {
					return err
				}

				errorPlainBz, err := wasmCtx.Decrypt(errorCipherBz, nonce)
				if err != nil {
					return err
				}

				err = json.Unmarshal(errorPlainBz, &answer.OutputError)
				if err != nil {
					return err
				}
			} else if strings.Contains(result.RawLog, "EnclaveErr") {
				answer.PlaintextError = result.RawLog
			}

			return cliCtx.PrintOutput(answer)
		},
	}

	return cmd
}

var contractErrorRegex = regexp.MustCompile(`wasm contract failed: generic: (.+)`)

func GetCmdGetContractStateSmart(cdc *codec.Codec) *cobra.Command {
	decoder := newArgDecoder(asciiDecodeString)

	cmd := &cobra.Command{
		Use:   "smart [bech32_address] [query]", // TODO add --from wallet
		Short: "Calls contract with given address  with query data and prints the returned result",
		Long:  "Calls contract with given address  with query data and prints the returned result",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)

			addr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}
			key := args[1]
			if key == "" {
				return errors.New("key must not be empty")
			}
			route := fmt.Sprintf("custom/%s/%s/%s/%s", types.QuerierRoute, keeper.QueryGetContractState, addr.String(), keeper.QueryMethodContractStateSmart)

			queryData, err := decoder.DecodeString(args[1])
			if err != nil {
				return fmt.Errorf("decode query: %s", err)
			}

			wasmCtx := wasmUtils.WASMContext{CLIContext: cliCtx}

			queryData, err = wasmCtx.Encrypt(queryData)
			if err != nil {
				return err
			}

			res, _, err := cliCtx.QueryWithData(route, queryData)
			if err != nil {
				return err
			}

			nonce := queryData[:32]
			resDecrypted := []byte{}
			if len(res) > 0 {
				resDecrypted, err = wasmCtx.Decrypt(res, nonce)
				if err != nil {
					return err
				}
			}

			decodedResp, err := base64.StdEncoding.DecodeString(string(resDecrypted))
			if err != nil {
				return err
			}

			fmt.Println(string(decodedResp))
			return nil
		},
	}
	decoder.RegisterFlags(cmd.PersistentFlags(), "query argument")
	return cmd
}

type argumentDecoder struct {
	// dec is the default decoder
	dec                func(string) ([]byte, error)
	asciiF, hexF, b64F bool
}

func newArgDecoder(def func(string) ([]byte, error)) *argumentDecoder {
	return &argumentDecoder{dec: def}
}

func (a *argumentDecoder) RegisterFlags(f *flag.FlagSet, argName string) {
	f.BoolVar(&a.asciiF, "ascii", false, "ascii encoded "+argName)
	f.BoolVar(&a.hexF, "hex", false, "hex encoded  "+argName)
	f.BoolVar(&a.b64F, "b64", false, "base64 encoded "+argName)
}

func (a *argumentDecoder) DecodeString(s string) ([]byte, error) {
	found := -1
	for i, v := range []*bool{&a.asciiF, &a.hexF, &a.b64F} {
		if !*v {
			continue
		}
		if found != -1 {
			return nil, errors.New("multiple decoding flags used")
		}
		found = i
	}
	switch found {
	case 0:
		return asciiDecodeString(s)
	case 1:
		return hex.DecodeString(s)
	case 2:
		return base64.StdEncoding.DecodeString(s)
	default:
		return a.dec(s)
	}
}

func asciiDecodeString(s string) ([]byte, error) {
	return []byte(s), nil
}
