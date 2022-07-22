package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/enigmampc/SecretNetwork/x/registration/internal/keeper"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/enigmampc/SecretNetwork/x/registration/internal/types"
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

// GetCmdListCode lists all wasm code uploaded
func GetCmdEncryptedSeed() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed [node-id]",
		Short: "Get encrypted seed for a node",
		Long:  "Get encrypted seed for a node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			nodeID := args[0]
			if len(nodeID) != types.PublicKeyLength {
				return fmt.Errorf("invalid Node ID format (req: hex string of length %d)", types.PublicKeyLength)
			}

			route := fmt.Sprintf("custom/%s/%s/%s", types.QuerierRoute, keeper.QueryEncryptedSeed, nodeID)
			res, _, err := clientCtx.Query(route)
			if err != nil {
				return err
			}
			fmt.Printf(fmt.Sprintf("0x%s", hex.EncodeToString(res)))
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
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			route := fmt.Sprintf("custom/%s/%s", types.QuerierRoute, keeper.QueryMasterCertificate)
			res, _, err := clientCtx.Query(route)
			if err != nil {
				return err
			}

			var certs types.GenesisState

			err = json.Unmarshal(res, &certs)
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(types.IoExchMasterCertPath, certs.IoMasterCertificate.Bytes, 0o644) //nolint:gosec // common cosmos issue, never works to fix it though.
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(types.NodeExchMasterCertPath, certs.NodeExchMasterCertificate.Bytes, 0o644) //nolint:gosec // common cosmos issue, never works to fix it though.
			if err != nil {
				return err
			}

			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
