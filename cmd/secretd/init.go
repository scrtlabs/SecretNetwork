package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cfg "github.com/cometbft/cometbft/config"
	"github.com/cosmos/go-bip39"
	"github.com/spf13/cobra"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math/unsafe"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/genutil/types"
)

const (
	// FlagOverwrite defines a flag to overwrite an existing genesis JSON file.
	FlagOverwrite = "overwrite"

	// FlagSeed defines a flag to initialize the private validator key from a specific seed.
	FlagRecover = "recover"

	// FlagDefaultBondDenom defines the default denom to use in the genesis file.
	FlagDefaultBondDenom = "default-denom"
)

type printInfo struct {
	Moniker    string          `json:"moniker" yaml:"moniker"`
	ChainID    string          `json:"chain_id" yaml:"chain_id"`
	NodeID     string          `json:"node_id" yaml:"node_id"`
	GenTxsDir  string          `json:"gentxs_dir" yaml:"gentxs_dir"`
	AppMessage json.RawMessage `json:"app_message" yaml:"app_message"`
}

func newPrintInfo(moniker, chainID, nodeID, genTxsDir string, appMessage json.RawMessage) printInfo {
	return printInfo{
		Moniker:    moniker,
		ChainID:    chainID,
		NodeID:     nodeID,
		GenTxsDir:  genTxsDir,
		AppMessage: appMessage,
	}
}

func displayInfo(info printInfo) error {
	out, err := json.MarshalIndent(info, "", " ")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(os.Stderr, "%s\n", out)

	return err
}

func InitCmd(mbm module.BasicManager, defaultNodeHome string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [moniker]",
		Short: "Initialize private validator, p2p, genesis, and application configuration files",
		Long:  `Initialize validators's and node's configuration files.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			cdc := clientCtx.Codec

			serverCtx := server.GetServerContextFromCmd(cmd)
			config := serverCtx.Config
			config.SetRoot(clientCtx.HomeDir)

			chainID, _ := cmd.Flags().GetString(flags.FlagChainID)
			switch {
			case chainID != "":
			case clientCtx.ChainID != "":
				chainID = clientCtx.ChainID
			default:
				chainID = fmt.Sprintf("test-chain-%v", unsafe.Str(6))
			}

			// only for mainnet
			if chainID == "secret-4" {
				seeds := []string{
					"6fb7169f7630da9468bf7cc0bcbbed1eb9ed0d7b@scrt-seed-01.scrtlabs.com:26656", // SCRT Labs 1
					"ab6394e953e0b570bb1deeb5a8b387aa0dc6188a@scrt-seed-02.scrtlabs.com:26656", // SCRT Labs 2
					"9cdaa5856e0245ecd73bd464308fb990fbc53b57@scrt-seed-03.scrtlabs.com:26656", // SCRT Labs 3
				}
				// Override default settings in config.toml
				config.P2P.Seeds = strings.Join(seeds, ",")
			}

			config.P2P.MaxNumInboundPeers = 320
			config.P2P.MaxNumOutboundPeers = 40
			config.Mempool.Size = 10000
			config.StateSync.TrustPeriod = 112 * time.Hour
			config.BlockSync.Version = "v0"

			// Get bip39 mnemonic
			var mnemonic string
			recover, _ := cmd.Flags().GetBool(FlagRecover)
			if recover {
				inBuf := bufio.NewReader(cmd.InOrStdin())
				value, err := input.GetString("Enter your bip39 mnemonic", inBuf)
				if err != nil {
					return err
				}

				mnemonic = value
				if !bip39.IsMnemonicValid(mnemonic) {
					return errors.New("invalid mnemonic")
				}
			}

			// Get initial height
			initHeight, _ := cmd.Flags().GetInt64(flags.FlagInitHeight)
			if initHeight < 1 {
				initHeight = 1
			}

			nodeID, _, err := genutil.InitializeNodeValidatorFilesFromMnemonic(config, mnemonic)
			if err != nil {
				return err
			}

			config.Moniker = args[0]

			genFile := config.GenesisFile()
			overwrite, _ := cmd.Flags().GetBool(FlagOverwrite)
			defaultDenom, _ := cmd.Flags().GetString(FlagDefaultBondDenom)

			// use os.Stat to check if the file exists
			_, err = os.Stat(genFile)
			if !overwrite && !os.IsNotExist(err) {
				return fmt.Errorf("genesis.json file already exists: %v", genFile)
			}

			// Overwrites the SDK default denom for side-effects
			if defaultDenom != "" {
				sdk.DefaultBondDenom = defaultDenom
			}
			appGenState := mbm.DefaultGenesis(cdc)

			appState, err := json.MarshalIndent(appGenState, "", " ")
			if err != nil {
				return errorsmod.Wrap(err, "Failed to marshal default genesis state")
			}

			appGenesis := &types.AppGenesis{
				AppName:       version.Version,
				AppVersion:    version.Version,
				ChainID:       chainID,
				AppState:      appState,
				InitialHeight: initHeight,
				Consensus: &types.ConsensusGenesis{
					Validators: nil,
				},
			}
			if _, err := os.Stat(genFile); err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			} else {
				appGenesis, err = types.AppGenesisFromFile(genFile)
				if err != nil {
					return errorsmod.Wrap(err, "Failed to read genesis doc from file")
				}
			}

			if err = genutil.ExportGenesisFile(appGenesis, genFile); err != nil {
				return errorsmod.Wrap(err, "Failed to export genesis file")
			}

			toPrint := newPrintInfo(config.Moniker, chainID, nodeID, "", appState)

			cfg.WriteConfigFile(filepath.Join(config.RootDir, "config", "config.toml"), config)
			return displayInfo(toPrint)
		},
	}

	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "node's home directory")
	cmd.Flags().BoolP(FlagOverwrite, "o", false, "overwrite the genesis.json file")
	cmd.Flags().Bool(FlagRecover, false, "provide seed phrase to recover existing key instead of creating")
	cmd.Flags().String(flags.FlagChainID, "", "genesis file chain-id, if left blank will be randomly created")
	cmd.Flags().String(FlagDefaultBondDenom, "", "genesis file default denomination, if left blank default value is 'stake'")
	cmd.Flags().Int64(flags.FlagInitHeight, 1, "specify the initial block height at genesis")

	return cmd
}
