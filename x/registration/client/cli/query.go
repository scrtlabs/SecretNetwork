package cli

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	flag "github.com/spf13/pflag"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
)

func GetQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the compute module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	queryCmd.AddCommand(
		GetCmdEncryptedSeed(),
		GetCmdMasterParams(),
	)
	return queryCmd
}

func GetCmdEncryptedSeed() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed [node-id]",
		Short: "Get encrypted seed for a node",
		Long:  "Get encrypted seed for a node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			grpcCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			nodeId := args[0]
			if len(nodeId) != types.PublicKeyLength {
				return fmt.Errorf("invalid Node ID format (req: hex string of length %d)", types.PublicKeyLength)
			}

			queryClient := types.NewQueryClient(grpcCtx)
			res, err := queryClient.EncryptedSeed(
				context.Background(),
				&types.QueryEncryptedSeedRequest{
					PubKey: []byte(nodeId),
				},
			)
			if err != nil {
				return sdkerrors.ErrNotFound.Wrapf("Failed to query seed for %s. Error: %s", args[0], err)
			}

			fmt.Printf("0x%s\n", hex.EncodeToString(res.EncryptedSeed))
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdListCode lists all wasm code uploaded
func GetCmdMasterParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret-network-params",
		Short: "Get parameters for the secret network",
		Long:  "Get parameters for the secret network - writes the parameters to [master-cert.der] by default",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			grpcCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(grpcCtx)
			res, err := queryClient.RegistrationKey(
				context.Background(),
				&emptypb.Empty{},
			)
			if err != nil {
				return sdkerrors.ErrNotFound.Wrapf("Failed to query master key. Error: %s", err)
			}

			var keys types.GenesisState

			err = json.Unmarshal(res.Key, &keys)
			if err != nil {
				return err
			}

			err = os.WriteFile(types.IoExchMasterKeyPath, []byte(base64.StdEncoding.EncodeToString(keys.IoMasterKey.Bytes)), 0o600)
			if err != nil {
				return err
			}

			err = os.WriteFile(types.NodeExchMasterKeyPath, []byte(base64.StdEncoding.EncodeToString(keys.NodeExchMasterKey.Bytes)), 0o600)
			if err != nil {
				return err
			}

			report, _ := json.Marshal(
				struct {
					Io_exch   string `json:"io-X-master-key"`
					Node_exch string `json:"node-X-master-key"`
				}{
					Io_exch:   base64.StdEncoding.EncodeToString(keys.IoMasterKey.Bytes),
					Node_exch: base64.StdEncoding.EncodeToString(keys.NodeExchMasterKey.Bytes),
				},
			)

			fmt.Printf("%s/n", string(report))

			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
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
