package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/gogo/protobuf/proto"

	cosmwasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	"github.com/spf13/cobra"

	"google.golang.org/protobuf/types/known/emptypb"

	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	wasmUtils "github.com/scrtlabs/SecretNetwork/x/compute/client/utils"
)

func GetQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the wasm module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
		SilenceUsage:               true,
	}
	queryCmd.AddCommand(
		GetCmdParams(),
		GetCmdListCode(),
		GetCmdListContractByCode(),
		GetCmdQueryCode(),
		GetCmdGetContractInfo(),
		GetQueryDecryptTxCmd(),
		GetCmdQueryLabel(),
		GetCmdCodeHashByContractAddress(),
		GetCmdGetContractStateSmart(),
		GetCmdCodeHashByCodeID(),
		GetCmdDecryptText(),
		GetCmdGetContractHistory(),
	)
	return queryCmd
}

// GetCmdParams lists all parameters of the compute module
func GetCmdParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "List all parameters of the compute module",
		Long:  "List all parameters of the compute module",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Params(cmd.Context(), &types.ParamsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(&res.Params)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdGetContractHistory prints the code history for a given contract
func GetCmdGetContractHistory() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "contract-history [bech32_address]",
		Short:   "Prints out the code history for a contract given its address",
		Long:    "Prints out the code history for a contract given its address",
		Aliases: []string{"history", "hist", "ch"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			_, err = sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.ContractHistory(
				context.Background(),
				&types.QueryContractHistoryRequest{
					ContractAddress: args[0],
				},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
		SilenceUsage: true,
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "contract history")
	return cmd
}

func GetCmdDecryptText() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decrypt [encrypted_data]",
		Short: "Attempt to decrypt an encrypted blob",
		Long: "Attempt to decrypt a base-64 encoded encrypted message. This is intended to be used if manual decrypt" +
			"is required for data that is unavailable to be decrypted using the 'query compute tx' command",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			encodedInput := args[0]

			dataCipherBz, err := base64.StdEncoding.DecodeString(encodedInput)
			if err != nil {
				return fmt.Errorf("error while trying to decode the encrypted output data from base64: %w", err)
			}

			nonce, originalTxSenderPubkey, ciphertextInput, err := parseEncryptedBlob(dataCipherBz)
			if err != nil {
				return fmt.Errorf("error while parsing encrypted blob: %w", err)
			}

			wasmCtx := wasmUtils.WASMContext{CLIContext: clientCtx}
			_, myPubkey, err := wasmCtx.GetTxSenderKeyPair()
			if err != nil {
				return fmt.Errorf("error while getting tx sender key pair: %w", err)
			}

			if !bytes.Equal(originalTxSenderPubkey, myPubkey) {
				return fmt.Errorf("cannot decrypt, not original tx sender")
			}

			dataPlaintextB64Bz, err := wasmCtx.Decrypt(ciphertextInput, nonce)
			if err != nil {
				return fmt.Errorf("error while trying to decrypt the output data: %w", err)
			}

			fmt.Printf("Decrypted data: %s\n", dataPlaintextB64Bz)
			return nil
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdCodeHashByID return the code hash of a contract by ID
func GetCmdCodeHashByCodeID() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contract-hash-by-id [code_id]",
		Short: "Return the code hash of a contract represented by ID",
		Long:  "Return the code hash of a contract represented by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			codeID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Code(
				context.Background(),
				&types.QueryByCodeIdRequest{
					CodeId: codeID,
				},
			)
			if err != nil {
				return err
			}

			fmt.Printf("0x%s\n", res.CodeHash)
			return nil
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func GetCmdQueryLabel() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label [label]",
		Short: "Check if a label is in use",
		Long:  "Check if a label is in use",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			res, err := GetContractAddressByLabel(args[0], clientCtx)
			if err != nil {
				if err == sdkerrors.ErrUnknownAddress {
					fmt.Printf("Label is available and not in use\n")
					return nil
				}

				return fmt.Errorf("error querying: %s", err)
			}

			fmt.Printf("Label is in use by contract address: %s\n", res)

			return nil
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryCode returns the bytecode for a given contract
func GetCmdQueryCode() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code [code_id] [output filename]",
		Short: "Downloads wasm bytecode for given code id",
		Long:  "Downloads wasm bytecode for given code id",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			codeID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Code(context.Background(), &types.QueryByCodeIdRequest{
				CodeId: codeID,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Downloading wasm code to %s\n", args[1])
			return os.WriteFile(args[1], res.Wasm, 0o600)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func GetCmdGetContractStateSmart() *cobra.Command {
	decoder := newArgDecoder(asciiDecodeString)
	cmd := &cobra.Command{
		Use:   "query [bech32_address] [query]",
		Short: "Calls contract with given address with query data and prints the returned result",
		Long:  "Calls contract with given address with query data and prints the returned result",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			contractAddr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return sdkerrors.ErrInvalidAddress.Wrapf("Invalid contract address: %s", args[0])
			}

			queryData, err := decoder.DecodeString(args[1])
			if err != nil {
				return err
			}

			return QueryWithData(contractAddr, queryData, clientCtx)
		},
		SilenceUsage: true,
	}
	decoder.RegisterFlags(cmd.PersistentFlags(), "key argument")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdListCode lists all wasm code uploaded
func GetCmdCodeHashByContractAddress() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contract-hash [address]",
		Short: "Return the code hash of a contract",
		Long:  "Return the code hash of a contract",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.CodeHashByContractAddress(
				context.Background(),
				&types.QueryByContractAddressRequest{
					ContractAddress: args[0],
				},
			)
			if err != nil {
				return fmt.Errorf("error querying contract hash: %s", err)
			}

			fmt.Printf("0x%s\n", res.CodeHash)
			return nil
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdListCode -> gRPC into x/compute/internal/keeper/querier.go: Codes(c context.Context, _ *empty.Empty)
func GetCmdListCode() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-code",
		Short:   "List all wasm bytecode on the chain",
		Long:    "List all wasm bytecode on the chain",
		Aliases: []string{"list-codes", "codes", "lco"},
		Args:    cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Codes(
				context.Background(),
				&emptypb.Empty{},
			)
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
		SilenceUsage: true,
	}
	flags.AddQueryFlagsToCmd(cmd)
	addPaginationFlags(cmd, "list codes")
	return cmd
}

// GetCmdListContractByCode lists all wasm code uploaded for given code id
func GetCmdListContractByCode() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-contract-by-code [code_id]",
		Short:   "List wasm all bytecode on the chain for given code id",
		Long:    "List wasm all bytecode on the chain for given code id",
		Aliases: []string{"list-contracts-by-code", "list-contracts", "contracts", "lca"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			codeID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}
			if codeID == 0 {
				return errors.New("empty code id")
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.ContractsByCodeId(
				context.Background(),
				&types.QueryByCodeIdRequest{
					CodeId: codeID,
				},
			)
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
		SilenceUsage: true,
	}
	flags.AddQueryFlagsToCmd(cmd)
	addPaginationFlags(cmd, "list contracts by code")
	return cmd
}

// GetCmdGetContractInfo gets details about a given contract
func GetCmdGetContractInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "contract [bech32_address]",
		Short:   "Prints out metadata of a contract given its address",
		Long:    "Prints out metadata of a contract given its address",
		Aliases: []string{"meta", "c"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			_, err = sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.ContractInfo(
				context.Background(),
				&types.QueryByContractAddressRequest{
					ContractAddress: args[0],
				},
			)
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
		SilenceUsage: true,
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func GetQueryDecryptTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx [hash]",
		Short: "Query for a transaction by hash in a committed block, decrypt input and outputs if I'm the tx sender",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			result, err := authtx.QueryTx(clientCtx, args[0])
			if err != nil {
				return err
			}

			if result.Empty() {
				return fmt.Errorf("no transaction found with hash %s", args[0])
			}

			txInputs := result.GetTx().GetMsgs()

			wasmCtx := wasmUtils.WASMContext{CLIContext: clientCtx}
			_, myPubkey, err := wasmCtx.GetTxSenderKeyPair()
			if err != nil {
				return fmt.Errorf("error in GetTxSenderKeyPair: %w", err)
			}

			answers := types.DecryptedAnswers{
				Answers:        make([]*types.DecryptedAnswer, len(txInputs)),
				OutputLogs:     []sdk.StringEvent{},
				OutputError:    "",
				PlaintextError: "",
			}
			nonces := make([][]byte, len(txInputs))

			for i, tx := range txInputs {
				var encryptedInput []byte
				answers.Answers[i] = &types.DecryptedAnswer{}

				switch txInput := tx.(type) {
				case *types.MsgExecuteContract:
					{
						encryptedInput = txInput.Msg
						answers.Answers[i].Type = "execute"
					}
				case *types.MsgInstantiateContract:
					{
						encryptedInput = txInput.InitMsg
						answers.Answers[i].Type = "instantiate"
					}
				}

				if encryptedInput != nil {
					nonce, originalTxSenderPubkey, ciphertextInput, err := parseEncryptedBlob(encryptedInput)
					if err != nil {
						return fmt.Errorf("can't parse encrypted blob: %w", err)
					}

					if !bytes.Equal(originalTxSenderPubkey, myPubkey) {
						return fmt.Errorf("cannot decrypt, not original tx sender")
					}

					var plaintextInput []byte
					if len(ciphertextInput) > 0 {
						plaintextInput, err = wasmCtx.Decrypt(ciphertextInput, nonce)
						if err != nil {
							return fmt.Errorf("error while trying to decrypt the tx input: %w", err)
						}
					}

					answers.Answers[i].Input = string(plaintextInput)
					nonces[i] = nonce
				}
			}

			dataOutputHexB64 := result.Data
			if dataOutputHexB64 != "" {
				dataOutputAsProtobuf, err := hex.DecodeString(dataOutputHexB64)
				if err != nil {
					return fmt.Errorf("error while trying to decode the encrypted output data from hex string: %w", err)
				}

				var txData sdk.TxMsgData
				err = proto.Unmarshal(dataOutputAsProtobuf, &txData)
				if err != nil {
					return fmt.Errorf("error while trying to parse data as protobuf: %w: %s", err, dataOutputHexB64)
				}

				for i, msgData := range txData.MsgResponses {
					if len(msgData.Value) != 0 {
						var dataField []byte
						switch {
						case msgData.TypeUrl == "/secret.compute.v1beta1.MsgInstantiateContractResponse":
							var msgResponse types.MsgInstantiateContractResponse
							err := proto.Unmarshal(msgData.Value, &msgResponse)
							if err != nil {
								continue
							}

							dataField = msgResponse.Data
						case msgData.TypeUrl == "/secret.compute.v1beta1.MsgExecuteContractResponse":
							var msgResponse types.MsgExecuteContractResponse
							err := proto.Unmarshal(msgData.Value, &msgResponse)
							if err != nil {
								continue
							}

							dataField = msgResponse.Data
						default:
							continue
						}

						dataPlaintextB64Bz, err := wasmCtx.Decrypt(dataField, nonces[i])
						if err != nil {
							continue
						}
						dataPlaintextB64 := string(dataPlaintextB64Bz)
						answers.Answers[i].OutputData = dataPlaintextB64

						dataPlaintext, err := base64.StdEncoding.DecodeString(dataPlaintextB64)
						if err != nil {
							continue
						}

						answers.Answers[i].OutputDataAsString = string(dataPlaintext)
					}
				}
			}

			// decrypt logs
			answers.OutputLogs = []sdk.StringEvent{}
			for _, e := range result.Events {
				if e.Type == "wasm" {
					for i, a := range e.Attributes {
						if a.Key != "contract_address" {
							// key
							if a.Key != "" {
								// Try to decrypt the log key. If it doesn't look encrypted, leave it as-is
								keyCiphertext, err := base64.StdEncoding.DecodeString(a.Key)
								if err != nil {
									continue
								}

								for _, nonce := range nonces {
									keyPlaintext, err := wasmCtx.Decrypt(keyCiphertext, nonce)
									if err != nil {
										continue
									}
									a.Key = string(keyPlaintext)
									break
								}
							}

							// value
							if a.Value != "" {
								// Try to decrypt the log value. If it doesn't look encrypted, leave it as-is
								valueCiphertext, err := base64.StdEncoding.DecodeString(a.Value)
								if err != nil {
									continue
								}
								for _, nonce := range nonces {
									valuePlaintext, err := wasmCtx.Decrypt(valueCiphertext, nonce)
									if err != nil {
										continue
									}
									a.Value = string(valuePlaintext)
									break
								}
							}
							e.Attributes[i] = a
						}
					}
					answers.OutputLogs = append(answers.OutputLogs, sdk.StringifyEvent(e))
				}
			}

			if types.IsEncryptedErrorCode(result.Code) && types.ContainsEncryptedString(result.RawLog) {
				for i, nonce := range nonces {
					stdErr, err := wasmCtx.DecryptError(result.RawLog, nonce)
					if err != nil {
						continue
					}
					answers.OutputError = string(append(json.RawMessage(fmt.Sprintf("message index %d: ", i)), stdErr...))
					break
				}
			} else if types.ContainsEnclaveError(result.RawLog) {
				answers.PlaintextError = result.RawLog
			}

			jsonBz, err := json.MarshalIndent(answers, "", "    ")
			if err != nil {
				return err
			}

			return clientCtx.PrintString(string(jsonBz))
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func QueryWithData(contractAddress sdk.AccAddress, queryData []byte, clientCtx client.Context) error {
	wasmCtx := wasmUtils.WASMContext{CLIContext: clientCtx}

	codeHash, err := GetCodeHashByContractAddr(clientCtx, contractAddress.String())
	if err != nil {
		return sdkerrors.ErrNotFound.Wrapf("Contract with address %s not found", contractAddress)
	}

	msg := types.SecretMsg{
		CodeHash: codeHash,
		Msg:      queryData,
	}

	queryData, err = wasmCtx.Encrypt(msg.Serialize())
	if err != nil {
		return err
	}
	nonce, _, _, _ := parseEncryptedBlob(queryData) //nolint:dogsled // Ignoring error since we just encrypted it

	queryClient := types.NewQueryClient(clientCtx)
	res, err := queryClient.QuerySecretContract(
		context.Background(),
		&types.QuerySecretContractRequest{
			ContractAddress: contractAddress.String(),
			Query:           queryData,
		},
	)
	if err != nil {
		if types.ErrContainsQueryError(err) {
			errorPlainBz, err := wasmCtx.DecryptError(err.Error(), nonce)
			if err != nil {
				return err
			}
			var stdErr cosmwasmTypes.StdError
			err = json.Unmarshal(errorPlainBz, &stdErr)
			if err != nil {
				return fmt.Errorf("query result: %s", string(errorPlainBz))
			}

			return fmt.Errorf("query result: %s", stdErr.Error())
		}
		// Itzik: Commenting this as it might have been a placeholder for encrypting
		// else if strings.Contains(err.Error(), "EnclaveErr") {
		//	return err
		//}
		return err
	}

	var resDecrypted []byte
	resDecrypted, err = wasmCtx.Decrypt(res.Data, nonce)
	if err != nil {
		return err
	}
	res.Data = resDecrypted
	decodedResp, err := base64.StdEncoding.DecodeString(string(resDecrypted))
	if err != nil {
		return err
	}

	fmt.Println(string(decodedResp))
	return nil
}

func GetContractAddressByLabel(label string, clientCtx client.Context) (string, error) {
	queryClient := types.NewQueryClient(clientCtx)
	response, err := queryClient.AddressByLabel(context.Background(), &types.QueryByLabelRequest{
		Label: label,
	})
	if err != nil {
		return "", err
	}
	return response.ContractAddress, nil
}

// supports a subset of the SDK pagination params for better resource utilization
func addPaginationFlags(cmd *cobra.Command, query string) {
	cmd.Flags().String(flags.FlagPageKey, "", fmt.Sprintf("pagination page-key of %s to query for", query))
	cmd.Flags().Uint64(flags.FlagLimit, 100, fmt.Sprintf("pagination limit of %s to query for", query))
	cmd.Flags().Bool(flags.FlagReverse, false, "results are sorted in descending order")
}
