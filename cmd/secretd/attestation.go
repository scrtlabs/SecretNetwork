package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/enigmampc/cosmos-sdk/types/module"
	"github.com/enigmampc/cosmos-sdk/x/genutil"
	app "github.com/enigmampc/SecretNetwork"
	"github.com/enigmampc/SecretNetwork/go-cosmwasm/api"
	reg "github.com/enigmampc/SecretNetwork/x/registration"
	ra "github.com/enigmampc/SecretNetwork/x/registration/remote_attestation"

	"github.com/enigmampc/cosmos-sdk/codec"
	"github.com/enigmampc/cosmos-sdk/server"
	"github.com/spf13/cobra"
)

func InitAttestation(
	_ *server.Context, _ *codec.Codec) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "init-enclave [output-file]",
		Short: "Perform remote attestation of the enclave",
		Long: `Create attestation report, signed by Intel which is used in the registation process of
the node to the chain. This process, if successful, will output a certificate which is used to authenticate with the 
blockchain. Writes the certificate in DER format to ~/attestation_cert
`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := api.KeyGen()
			if err != nil {
				return fmt.Errorf("failed to initialize enclave: %w", err)
			}

			_, err = api.CreateAttestationReport()
			if err != nil {
				return fmt.Errorf("failed to create attestation report: %w", err)
			}
			return nil
		},
	}

	return cmd
}

func InitBootstrapCmd(
	ctx *server.Context, cdc *codec.Codec, mbm module.BasicManager) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "init-bootstrap [node-exchange-file] [io-exchange-file]",
		Short: "Perform bootstrap initialization",
		Long: `Create attestation report, signed by Intel which is used in the registration process of
the node to the chain. This process, if successful, will output a certificate which is used to authenticate with the 
blockchain. Writes the certificate in DER format to ~/attestation_cert
`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			config := ctx.Config

			genFile := config.GenesisFile()
			appState, genDoc, err := genutil.GenesisStateFromGenFile(cdc, genFile)
			if err != nil {
				return fmt.Errorf("failed to unmarshal genesis state: %w", err)
			}

			regGenState := reg.GetGenesisStateFromAppState(cdc, appState)

			// the master key of the generated certificate is returned here
			masterKey, err := api.InitBootstrap()
			if err != nil {
				return fmt.Errorf("failed to initialize enclave: %w", err)
			}

			userHome, _ := os.UserHomeDir()

			// Load consensus_seed_exchange_pubkey
			cert := []byte(nil)
			if len(args) >= 1 {
				cert, err = ioutil.ReadFile(args[0])
				if err != nil {
					return err
				}
			} else {
				cert, err = ioutil.ReadFile(filepath.Join(userHome, reg.NodeExchMasterCertPath))
				if err != nil {
					return err
				}
			}

			pubkey, err := ra.VerifyRaCert(cert)
			if err != nil {
				return err
			}

			fmt.Println(fmt.Sprintf("%s", hex.EncodeToString(pubkey)))
			fmt.Println(fmt.Sprintf("%s", hex.EncodeToString(masterKey)))

			// sanity check - make sure the certificate we're using matches the generated key
			if hex.EncodeToString(pubkey) != hex.EncodeToString(masterKey) {
				return fmt.Errorf("invalid certificate for master public key")
			}

			regGenState.NodeExchMasterCertificate = cert

			// Load consensus_io_exchange_pubkey
			if len(args) == 2 {
				cert, err = ioutil.ReadFile(args[1])
				if err != nil {
					return err
				}
			} else {
				cert, err = ioutil.ReadFile(filepath.Join(userHome, reg.IoExchMasterCertPath))
				if err != nil {
					return err
				}
			}
			regGenState.IoMasterCertificate = cert

			// Create genesis state from certificates
			regGenStateBz, err := cdc.MarshalJSON(regGenState)
			if err != nil {
				return fmt.Errorf("failed to marshal auth genesis state: %w", err)
			}

			appState[reg.ModuleName] = regGenStateBz

			appStateJSON, err := cdc.MarshalJSON(appState)
			if err != nil {
				return fmt.Errorf("failed to marshal application genesis state: %w", err)
			}

			genDoc.AppState = appStateJSON
			return genutil.ExportGenesisFile(genDoc, genFile)
		},
	}

	return cmd
}

func ParseCert(_ *server.Context, _ *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parse [cert file]",
		Short: "Verify and parse a certificate file",
		Long: "Helper to verify generated credentials, and extract the public key of the secret node, which is used to" +
			"register the node, during node initialization",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			// parse coins trying to be sent
			cert, err := ioutil.ReadFile(args[0])
			if err != nil {
				return err
			}

			pubkey, err := ra.VerifyRaCert(cert)
			if err != nil {
				return err
			}

			fmt.Println(fmt.Sprintf("0x%s", hex.EncodeToString(pubkey)))
			return nil
		},
	}

	return cmd
}

func ConfigureSecret(_ *server.Context, _ *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use: "configure-secret [master-cert] [seed]",
		Short: "After registration is successful, configure the secret node with the credentials file and the encrypted" +
			"seed that was written on-chain",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

			// parse coins trying to be sent
			cert, err := ioutil.ReadFile(args[0])
			if err != nil {
				return err
			}

			// We expect seed to be 48 bytes of encrypted data (aka 96 hex chars) [32 bytes + 12 IV]
			seed := args[1]
			if len(seed) != reg.EncryptedKeyLength || !reg.IsHexString(seed) {
				return fmt.Errorf("invalid encrypted seed format (requires hex string of length 96 without 0x prefix)")
			}

			cfg := reg.SeedConfig{
				EncryptedKey: seed,
				MasterCert:   base64.StdEncoding.EncodeToString(cert),
			}

			cfgBytes, err := json.Marshal(&cfg)
			if err != nil {
				return err
			}

			path := filepath.Join(app.DefaultNodeHome, reg.SecretNodeCfgFolder, reg.SecretNodeSeedConfig)
			// fmt.Println("File Created Successfully", path)
			if os.IsNotExist(err) {
				var file, err = os.Create(path)
				if err != nil {
					return fmt.Errorf("failed to open config file '%s': %w", path, err)
				}
				_ = file.Close()
			}

			err = ioutil.WriteFile(path, cfgBytes, 0644)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func HealthCheck(_ *server.Context, _ *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-enclave",
		Short: "Test enclave status",
		Long: "Help diagnose issues by performing a basic sanity test that SGX is working properly",
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {

			res, err := api.HealthCheck()
			if err != nil {
				return fmt.Errorf("failed to start enclave. Enclave returned: %s", err)
			}

			fmt.Println(fmt.Sprintf("SGX enclave health status: %s", res))
			return nil
		},
	}

	return cmd
}

func ResetEnclave(_ *server.Context, _ *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset-enclave",
		Short: "Reset registration & enclave parameters",
		Long: "This will delete all registration and enclave parameters. Use when something goes wrong and you want to start fresh." +
			"You will have to go through registration again to be able to start the node",
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {

			// remove .secretd/.node/seed.json
			path := filepath.Join(app.DefaultNodeHome, reg.SecretNodeCfgFolder, reg.SecretNodeSeedConfig)
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				fmt.Printf("Removing %s\n", path)
				err = os.Remove(path)
				if err != nil {
					return err
				}
			} else {
				if err != nil {
					println(err.Error())
				}
			}


			// remove sgx_secrets
			sgxSecretsDir := os.Getenv("SCRT_SGX_STORAGE")
			if sgxSecretsDir == "" {
				sgxSecretsDir = os.ExpandEnv("$HOME/.sgx_secrets")
			}
			if _, err := os.Stat(sgxSecretsDir); !os.IsNotExist(err) {
				fmt.Printf("Removing %s\n", sgxSecretsDir)
				err = os.RemoveAll(sgxSecretsDir)
				if err != nil {
					return err
				}
				err := os.MkdirAll(sgxSecretsDir, 644)
				if err != nil {
					return err
				}
			} else {
				if err != nil {
					println(err.Error())
				}
			}
			return nil
		},
	}

	return cmd
}