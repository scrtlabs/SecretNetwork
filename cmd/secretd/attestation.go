//go:build !secretcli
// +build !secretcli

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
	reg "github.com/scrtlabs/SecretNetwork/x/registration"
	ra "github.com/scrtlabs/SecretNetwork/x/registration/remote_attestation"
	"github.com/spf13/cobra"
)

const (
	flagReset                     = "reset"
	flagPulsar                    = "pulsar"
	flagCustomRegistrationService = "registration-service"
)

const (
	flagLegacyRegistrationNode = "registration-node"
	flagLegacyBootstrapNode    = "node"
)

const (
	mainnetRegistrationService = "https://mainnet-register.scrtlabs.com/api/registernode"
	pulsarRegistrationService  = "https://testnet-register.scrtlabs.com/api/registernode"
)

func InitAttestation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init-enclave [output-file]",
		Short: "Perform remote attestation of the enclave",
		Long: `Create attestation report, signed by Intel which is used in the registation process of
the node to the chain. This process, if successful, will output a certificate which is used to authenticate with the 
blockchain. Writes the certificate in DER format to ~/attestation_cert
`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			sgxSecretsDir := os.Getenv("SCRT_SGX_STORAGE")
			if sgxSecretsDir == "" {
				sgxSecretsDir = os.ExpandEnv("/opt/secret/.sgx_secrets")
			}

			// create sgx secrets dir if it doesn't exist
			if _, err := os.Stat(sgxSecretsDir); !os.IsNotExist(err) {
				err := os.MkdirAll(sgxSecretsDir, 0o777)
				if err != nil {
					return err
				}
			}

			sgxSecretsPath := sgxSecretsDir + string(os.PathSeparator) + reg.EnclaveRegistrationKey

			resetFlag, err := cmd.Flags().GetBool(flagReset)
			if err != nil {
				return fmt.Errorf("error with reset flag: %s", err)
			}

			if !resetFlag {
				if _, err := os.Stat(sgxSecretsPath); os.IsNotExist(err) {
					fmt.Println("Creating new enclave registration key")
					_, err := api.KeyGen()
					if err != nil {
						return fmt.Errorf("failed to initialize enclave: %w", err)
					}
				} else {
					fmt.Println("Enclave key already exists. If you wish to overwrite and reset the node, use the --reset flag")
				}
			} else {
				fmt.Println("Reset enclave flag set, generating new enclave registration key. You must now re-register the node")
				_, err := api.KeyGen()
				if err != nil {
					return fmt.Errorf("failed to initialize enclave: %w", err)
				}
			}

			apiKeyFile, err := reg.GetApiKey()
			if err != nil {
				return fmt.Errorf("failed to initialize enclave: %w", err)
			}

			_, err = api.CreateAttestationReport(apiKeyFile, false)
			if err != nil {
				return fmt.Errorf("failed to create attestation report: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().Bool(flagReset, false, "Optional flag to regenerate the enclave registration key")

	return cmd
}

func InitBootstrapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init-bootstrap [node-exchange-file] [io-exchange-file]",
		Short: "Perform bootstrap initialization",
		Long: `Create attestation report, signed by Intel which is used in the registration process of
the node to the chain. This process, if successful, will output a certificate which is used to authenticate with the 
blockchain. Writes the certificate in DER format to ~/attestation_cert
`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			cdc := clientCtx.Codec

			serverCtx := server.GetServerContextFromCmd(cmd)
			config := serverCtx.Config

			genFile := config.GenesisFile()
			appState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genFile)
			if err != nil {
				return fmt.Errorf("failed to unmarshal genesis state: %w", err)
			}

			regGenState := reg.GetGenesisStateFromAppState(cdc, appState)

			spidFile, err := reg.GetSpid()
			if err != nil {
				return fmt.Errorf("failed to initialize enclave: %w", err)
			}

			apiKeyFile, err := reg.GetApiKey()
			if err != nil {
				return fmt.Errorf("failed to initialize enclave: %w", err)
			}

			// the master key of the generated certificate is returned here
			masterKey, err := api.InitBootstrap(spidFile, apiKeyFile)
			if err != nil {
				return fmt.Errorf("failed to initialize enclave: %w", err)
			}

			userHome, _ := os.UserHomeDir()

			// Load consensus_seed_exchange_pubkey
			var key []byte
			if len(args) >= 1 {
				key, err = os.ReadFile(args[0])
				if err != nil {
					return err
				}
			} else {
				key, err = os.ReadFile(filepath.Join(userHome, reg.NodeExchMasterKeyPath))
				if err != nil {
					return err
				}
			}

			pubkey, err := base64.StdEncoding.DecodeString(string(key))
			if err != nil {
				return err
			}

			fmt.Printf("%s\n", hex.EncodeToString(pubkey))
			fmt.Printf("%s\n", hex.EncodeToString(masterKey))

			// sanity check - make sure the certificate we're using matches the generated key
			if hex.EncodeToString(pubkey) != hex.EncodeToString(masterKey) {
				return fmt.Errorf("invalid certificate for master public key")
			}

			regGenState.NodeExchMasterKey.Bytes, err = base64.StdEncoding.DecodeString(string(key))
			if err != nil {
				return err
			}

			// Load consensus_io_exchange_pubkey
			if len(args) == 2 {
				key, err = os.ReadFile(args[1])
				if err != nil {
					return err
				}
			} else {
				key, err = os.ReadFile(filepath.Join(userHome, reg.IoExchMasterKeyPath))
				if err != nil {
					return err
				}
			}

			regGenState.IoMasterKey.Bytes, err = base64.StdEncoding.DecodeString(string(key))
			if err != nil {
				return err
			}

			// Create genesis state from certificates
			regGenStateBz, err := cdc.MarshalJSON(&regGenState)
			if err != nil {
				return fmt.Errorf("failed to marshal auth genesis state: %w", err)
			}

			appState[reg.ModuleName] = regGenStateBz

			appStateJSON, err := json.Marshal(appState)
			if err != nil {
				return fmt.Errorf("failed to marshal application genesis state: %w", err)
			}

			genDoc.AppState = appStateJSON
			return genutil.ExportGenesisFile(genDoc, genFile)
		},
	}

	return cmd
}

func ParseCert() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parse [cert file]",
		Short: "Verify and parse a certificate file",
		Long: "Helper to verify generated credentials, and extract the public key of the secret node, which is used to" +
			"register the node, during node initialization",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// parse coins trying to be sent
			cert, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			pubkey, err := ra.UNSAFE_VerifyRaCert(cert)
			if err != nil {
				return err
			}

			fmt.Printf("0x%s\n", hex.EncodeToString(pubkey))
			return nil
		},
	}

	return cmd
}

func ConfigureSecret() *cobra.Command {
	cmd := &cobra.Command{
		Use: "configure-secret [master-key] [seed]",
		Short: "After registration is successful, configure the secret node with the master key file and the encrypted " +
			"seed that was written on-chain",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			masterKey, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			seed := args[1]
			println(seed)
			if (len(seed) != reg.LegacyEncryptedKeyLength && len(seed) != reg.EncryptedKeyLength) || !reg.IsHexString(seed) {
				return fmt.Errorf("invalid encrypted seed format (requires hex string of length of at least 96 bytes without 0x prefix)")
			}

			cfg := reg.SeedConfig{
				EncryptedKey: seed,
				MasterKey:    string(masterKey),
				Version:      reg.SeedConfigVersion,
			}

			cfgBytes, err := json.Marshal(&cfg)
			if err != nil {
				return err
			}

			homeDir, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return err
			}

			// Create .secretd/.node directory if it doesn't exist
			nodeDir := filepath.Join(homeDir, reg.SecretNodeCfgFolder)
			err = os.MkdirAll(nodeDir, os.ModePerm)
			if err != nil {
				return err
			}

			seedFilePath := filepath.Join(nodeDir, reg.SecretNodeSeedNewConfig)

			err = os.WriteFile(seedFilePath, cfgBytes, 0o600)
			if err != nil {
				return err
			}

			createOldSecret(seed, nodeDir)

			return nil
		},
	}

	return cmd
}

func createOldSecret(combinedSeed string, nodeDir string) error {
	seedFilePath := filepath.Join(nodeDir, reg.SecretNodeSeedLegacyConfig)
	if _, err := os.Stat(seedFilePath); err == nil {
		return nil
	}

	if len(combinedSeed) != reg.EncryptedKeyLength || !reg.IsHexString(combinedSeed) {
		return fmt.Errorf("invalid encrypted seed format (requires hex string of length 192 without 0x prefix)")
	}

	seed := combinedSeed[0:reg.LegacyEncryptedKeyLength]
	println(seed)

	cfg := reg.LegacySeedConfig{
		EncryptedKey: seed,
		MasterCert:   reg.LegacyIoMasterCertificate,
	}

	cfgBytes, err := json.Marshal(&cfg)
	if err != nil {
		return err
	}

	err = os.WriteFile(seedFilePath, cfgBytes, 0o600)
	if err != nil {
		return err
	}

	return nil
}

func HealthCheck() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-enclave",
		Short: "Test enclave status",
		Long:  "Help diagnose issues by performing a basic sanity test that SGX is working properly",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := api.HealthCheck()
			if err != nil {
				return fmt.Errorf("failed to start enclave. Enclave returned: %s", err)
			}

			fmt.Printf("SGX enclave health status: %s\n", res)
			return nil
		},
	}

	return cmd
}

func ResetEnclave() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset-enclave",
		Short: "Reset registration & enclave parameters",
		Long: "This will delete all registration and enclave parameters. Use when something goes wrong and you want to start fresh." +
			"You will have to go through registration again to be able to start the node",
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return err
			}

			// Remove .secretd/.node/seed.json
			path := filepath.Join(homeDir, reg.SecretNodeCfgFolder, reg.SecretNodeSeedNewConfig)
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				fmt.Printf("Removing %s\n", path)
				err = os.Remove(path)
				if err != nil {
					return err
				}
			} else if err != nil {
				println(err.Error())
			}

			// remove sgx_secrets
			sgxSecretsDir := os.Getenv("SCRT_SGX_STORAGE")
			if sgxSecretsDir == "" {
				sgxSecretsDir = os.ExpandEnv("/opt/secret/.sgx_secrets")
			}
			if _, err := os.Stat(sgxSecretsDir); !os.IsNotExist(err) {
				fmt.Printf("Removing %s\n", sgxSecretsDir)
				err = os.RemoveAll(sgxSecretsDir)
				if err != nil {
					return err
				}
				err := os.MkdirAll(sgxSecretsDir, 0o777)
				if err != nil {
					return err
				}
			} else if err != nil {
				println(err.Error())
			}
			return nil
		},
	}

	return cmd
}

type OkayResponse struct {
	Status          string `json:"status"`
	Details         KeyVal `json:"details"`
	RegistrationKey string `json:"registration_key"`
}

type KeyVal struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ErrorResponse struct {
	Status  string `json:"status"`
	Details string `json:"details"`
}

// AutoRegisterNode *** EXPERIMENTAL ***
func AutoRegisterNode() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auto-register",
		Short: "Perform remote attestation of the enclave",
		Long: `Automatically handles all registration processes. ***EXPERIMENTAL***
Please report any issues with this command
`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			sgxSecretsFolder := os.Getenv("SCRT_SGX_STORAGE")
			if sgxSecretsFolder == "" {
				sgxSecretsFolder = os.ExpandEnv("/opt/secret/.sgx_secrets")
			}

			sgxEnclaveKeyPath := filepath.Join(sgxSecretsFolder, reg.EnclaveRegistrationKey)
			sgxAttestationCert := filepath.Join(sgxSecretsFolder, reg.AttestationCertPath)

			resetFlag, err := cmd.Flags().GetBool(flagReset)
			if err != nil {
				return fmt.Errorf("error with reset flag: %s", err)
			}

			if !resetFlag {
				if _, err := os.Stat(sgxEnclaveKeyPath); os.IsNotExist(err) {
					fmt.Println("Creating new enclave registration key")
					_, err := api.KeyGen()
					if err != nil {
						return fmt.Errorf("failed to initialize enclave: %w", err)
					}
				} else {
					fmt.Println("Enclave key already exists. If you wish to overwrite and reset the node, use the --reset flag")
					return nil
				}
			} else {
				fmt.Println("Reset enclave flag set, generating new enclave registration key. You must now re-register the node")
				_, err := api.KeyGen()
				if err != nil {
					return fmt.Errorf("failed to initialize enclave: %w", err)
				}
			}

			apiKeyFile, err := reg.GetApiKey()
			if err != nil {
				return fmt.Errorf("failed to initialize enclave: %w", err)
			}

			_, err = api.CreateAttestationReport(apiKeyFile, false)
			if err != nil {
				return fmt.Errorf("failed to create attestation report: %w", err)
			}

			// read the attestation certificate that we just created
			cert, err := os.ReadFile(sgxAttestationCert)
			if err != nil {
				_ = os.Remove(sgxAttestationCert)
				return err
			}

			_ = os.Remove(sgxAttestationCert)

			// verify certificate
			_, err = ra.UNSAFE_VerifyRaCert(cert)
			if err != nil {
				return err
			}

			regUrl := mainnetRegistrationService

			pulsarFlag, err := cmd.Flags().GetBool(flagPulsar)
			if err != nil {
				return fmt.Errorf("error with testnet flag: %s", err)
			}

			// register the node
			customRegUrl, err := cmd.Flags().GetString(flagCustomRegistrationService)
			if err != nil {
				return err
			}

			if pulsarFlag { //nolint:gocritic
				regUrl = pulsarRegistrationService
				log.Println("Registering node on Pulsar testnet")
			} else if customRegUrl != "" {
				regUrl = customRegUrl
				log.Println("Registering node with custom registration service")
			} else {
				log.Println("Registering node on mainnet")
			}

			// call registration service to register us
			data := []byte(fmt.Sprintf(`{
				"certificate": "%s"
			}`, base64.StdEncoding.EncodeToString(cert)))

			resp, err := http.Post(regUrl, "application/json", bytes.NewBuffer(data))
			if err != nil {
				log.Fatalln(err)
			}

			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatalln(err)
			}

			if resp.StatusCode != http.StatusOK {
				errDetails := ErrorResponse{}
				fmt.Println(string(body))
				err := json.Unmarshal(body, &errDetails)
				if err != nil {
					return fmt.Errorf(fmt.Sprintf("Registration TX was not successful - %s", err))
				}
				return fmt.Errorf(fmt.Sprintf("Registration TX was not successful - %s", errDetails.Details))
			}

			details := OkayResponse{}
			err = json.Unmarshal(body, &details)
			if err != nil {
				return fmt.Errorf(fmt.Sprintf("Error getting seed from registration service - %s", err))
			}

			seed := details.Details.Value
			log.Printf(`seed: %s\n`, seed)

			if len(seed) > 2 {
				seed = seed[2:]
			}

			if len(seed) != reg.EncryptedKeyLength || !reg.IsHexString(seed) {
				return fmt.Errorf("invalid encrypted seed format (requires hex string of length 148 without 0x prefix)")
			}

			regPublicKey := details.RegistrationKey

			// We expect seed to be 48 bytes of encrypted data (aka 96 hex chars) [32 bytes + 12 IV]

			cfg := reg.SeedConfig{
				EncryptedKey: seed,
				MasterKey:    regPublicKey,
				Version:      reg.SeedConfigVersion,
			}

			cfgBytes, err := json.Marshal(&cfg)
			if err != nil {
				return err
			}

			homeDir, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return err
			}

			seedCfgFile := filepath.Join(homeDir, reg.SecretNodeCfgFolder, reg.SecretNodeSeedNewConfig)
			seedCfgDir := filepath.Join(homeDir, reg.SecretNodeCfgFolder)

			// create seed directory if it doesn't exist
			_, err = os.Stat(seedCfgDir)
			if os.IsNotExist(err) {
				err = os.MkdirAll(seedCfgDir, 0o777)
				if err != nil {
					return fmt.Errorf("failed to create directory '%s': %w", seedCfgDir, err)
				}
			}

			// write seed to file - if file doesn't exist, write it. If it does, delete the existing one and create this
			_, err = os.Stat(seedCfgFile)
			if os.IsNotExist(err) {
				err = os.WriteFile(seedCfgFile, cfgBytes, 0o600)
				if err != nil {
					return err
				}
			} else {
				err = os.Remove(seedCfgFile)
				if err != nil {
					return fmt.Errorf("failed to modify file '%s': %w", seedCfgFile, err)
				}

				err = os.WriteFile(seedCfgFile, cfgBytes, 0o600)
				if err != nil {
					return fmt.Errorf("failed to create file '%s': %w", seedCfgFile, err)
				}
			}

			createOldSecret(seed, seedCfgDir)

			fmt.Println("Done registering! Ready to start...")
			return nil
		},
	}
	cmd.Flags().Bool(flagReset, false, "Optional flag to regenerate the enclave registration key")
	cmd.Flags().Bool(flagPulsar, false, "Set --pulsar flag if registering with the Pulsar testnet")
	cmd.Flags().String(flagCustomRegistrationService, "", "Use this flag if you wish to specify a custom registration service")

	cmd.Flags().String(flagLegacyBootstrapNode, "", "DEPRECATED: This flag is no longer required or in use")
	cmd.Flags().String(flagLegacyRegistrationNode, "", "DEPRECATED: This flag is no longer required or in use")

	return cmd
}
