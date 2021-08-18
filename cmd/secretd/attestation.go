// +build !secretcli

package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	app2 "github.com/enigmampc/SecretNetwork/app"
	"github.com/enigmampc/SecretNetwork/go-cosmwasm/api"
	reg "github.com/enigmampc/SecretNetwork/x/registration"
	ra "github.com/enigmampc/SecretNetwork/x/registration/remote_attestation"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const flagReset = "reset"
const flagRegistrationService = "registration-node"
const flagBootstrapNode = "node"

type WrappedSeedResponse struct {
	Height string       `json:"height"`
	Result SeedResponse `json:"result"`
}

type WrappedRegistrationResponse struct {
	Height string               `json:"height"`
	Result RegistrationResponse `json:"result"`
}

type RegistrationResponse struct {
	RegistrationKey []byte `json:"RegistrationKey"`
}

type SeedResponse struct {
	Seed string `json:"Seed"`
}

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
				err := os.MkdirAll(sgxSecretsDir, 0777)
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

			spidFile, err := Asset("spid.txt")
			if err != nil {
				return fmt.Errorf("failed to initialize enclave: %w", err)
			}

			apiKeyFile, err := Asset("api_key.txt")
			if err != nil {
				return fmt.Errorf("failed to initialize enclave: %w", err)
			}

			_, err = api.CreateAttestationReport(spidFile, apiKeyFile)
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
			depCdc := clientCtx.Codec
			cdc := depCdc.(codec.Codec)

			serverCtx := server.GetServerContextFromCmd(cmd)
			config := serverCtx.Config

			genFile := config.GenesisFile()
			appState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genFile)
			if err != nil {
				return fmt.Errorf("failed to unmarshal genesis state: %w", err)
			}

			regGenState := reg.GetGenesisStateFromAppState(cdc, appState)

			spidFile, err := Asset("spid.txt")
			if err != nil {
				return fmt.Errorf("failed to initialize enclave: %w", err)
			}

			apiKeyFile, err := Asset("api_key.txt")
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

			regGenState.NodeExchMasterCertificate.Bytes = cert

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
			regGenState.IoMasterCertificate.Bytes = cert

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

func ConfigureSecret() *cobra.Command {
	cmd := &cobra.Command{
		Use: "configure-secret [master-cert] [seed]",
		Short: "After registration is successful, configure the secret node with the credentials file and the encrypted " +
			"seed that was written on-chain",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

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

			path := filepath.Join(app2.DefaultNodeHome, reg.SecretNodeCfgFolder, reg.SecretNodeSeedConfig)
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

			fmt.Println(fmt.Sprintf("SGX enclave health status: %s", res))
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

			// remove .secretd/.node/seed.json
			path := filepath.Join(app2.DefaultNodeHome, reg.SecretNodeCfgFolder, reg.SecretNodeSeedConfig)
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
				sgxSecretsDir = os.ExpandEnv("/opt/secret/.sgx_secrets")
			}
			if _, err := os.Stat(sgxSecretsDir); !os.IsNotExist(err) {
				fmt.Printf("Removing %s\n", sgxSecretsDir)
				err = os.RemoveAll(sgxSecretsDir)
				if err != nil {
					return err
				}
				err := os.MkdirAll(sgxSecretsDir, 0777)
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

			spidFile, err := Asset("spid.txt")
			if err != nil {
				return fmt.Errorf("failed to initialize enclave: %w", err)
			}

			apiKeyFile, err := Asset("api_key.txt")
			if err != nil {
				return fmt.Errorf("failed to initialize enclave: %w", err)
			}

			_, err = api.CreateAttestationReport(spidFile, apiKeyFile)
			if err != nil {
				return fmt.Errorf("failed to create attestation report: %w", err)
			}

			// read the attestation certificate that we just created
			cert, err := ioutil.ReadFile(sgxAttestationCert)
			if err != nil {
				_ = os.Remove(sgxAttestationCert)
				return err
			}

			_ = os.Remove(sgxAttestationCert)

			// verify certificate
			pubKey, err := ra.VerifyRaCert(cert)
			if err != nil {
				return err
			}

			// register the node
			regUrl, err := cmd.Flags().GetString(flagRegistrationService)
			if err != nil {
				return err
			}

			// call registration service to register us
			//data := url.Values{
			//	"cert": {base64.StdEncoding.EncodeToString(cert)},
			//}
			resp, err := http.Get(fmt.Sprintf(`%s/register?cert=%s`, regUrl, url.QueryEscape(base64.StdEncoding.EncodeToString(cert))))
			if err != nil {
				log.Fatalln(err)
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatalln(err)
			}

			//Convert the body to type string and extract the seed
			txHash := string(body)
			txHash = txHash[1 : len(txHash)-1]
			txHash = removeQuotes(txHash)
			if len(txHash) != 64 || !reg.IsHexString(txHash) {
				return fmt.Errorf(fmt.Sprintf("Registration TX was not successful - %s", txHash))
			}

			bootstrapNode, err := cmd.Flags().GetString(flagBootstrapNode)
			if err != nil {
				return err
			}
			log.Printf("Waiting for on-chain Register...")
			time.Sleep(10 * time.Second)

			log.Printf("Getting encrypted seed")
			log.Printf(fmt.Sprintf(`requesting: %s/reg/seed/%s`, bootstrapNode, hex.EncodeToString(pubKey)))
			// get encrypted seed for our node
			resp, err = http.Get(fmt.Sprintf(`%s/reg/seed/%s`, bootstrapNode, hex.EncodeToString(pubKey)))
			if err != nil {
				log.Fatalln(err)
			}

			body, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatalln(err)
			}

			seedResponse := WrappedSeedResponse{}
			//Convert the body to type string
			err = json.Unmarshal(body, &seedResponse)
			if err != nil {
				log.Fatalln(err)
			}

			seed := seedResponse.Result.Seed
			log.Printf(fmt.Sprintf(`seed: %s`, seed))

			if len(seed) != reg.EncryptedKeyLength || !reg.IsHexString(seed) {
				return fmt.Errorf("invalid encrypted seed format (requires hex string of length 96 without 0x prefix)")
			}

			// get network registration public key
			resp, err = http.Get(fmt.Sprintf(`%s/reg/registration-key`, bootstrapNode))
			if err != nil {
				log.Fatalln(err)
			}

			body, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatalln(err)
			}

			regResponse := WrappedRegistrationResponse{}
			//Convert the body to type string
			err = json.Unmarshal(body, &regResponse)
			if err != nil {
				log.Fatalln(err)
			}

			regPublicKey := regResponse.Result.RegistrationKey

			// We expect seed to be 48 bytes of encrypted data (aka 96 hex chars) [32 bytes + 12 IV]

			cfg := reg.SeedConfig{
				EncryptedKey: seed,
				MasterCert:   base64.StdEncoding.EncodeToString(regPublicKey),
			}

			cfgBytes, err := json.Marshal(&cfg)
			if err != nil {
				return err
			}

			seedCfgFile := filepath.Join(app2.DefaultNodeHome, reg.SecretNodeCfgFolder, reg.SecretNodeSeedConfig)
			seedCfgDir := filepath.Join(app2.DefaultNodeHome, reg.SecretNodeCfgFolder)

			// create seed directory if it doesn't exist
			_, err = os.Stat(seedCfgDir)
			if os.IsNotExist(err) {
				err = os.MkdirAll(seedCfgDir, 0777)
				if err != nil {
					return fmt.Errorf("failed to create directory '%s': %w", seedCfgDir, err)
				}
			}

			// write seed to file - if file doesn't exist, write it. If it does, delete the existing one and create this
			_, err = os.Stat(seedCfgFile)
			if os.IsNotExist(err) {
				err = ioutil.WriteFile(seedCfgFile, cfgBytes, 0644)
				if err != nil {
					return err
				}
			} else {
				err = os.Remove(seedCfgFile)
				if err != nil {
					return fmt.Errorf("failed to modify file '%s': %w", seedCfgFile, err)
				}

				err = ioutil.WriteFile(seedCfgFile, cfgBytes, 0644)
				if err != nil {
					return fmt.Errorf("failed to create file '%s': %w", seedCfgFile, err)
				}
			}

			fmt.Println("Done registering! Ready to start...")
			return nil
		},
	}
	cmd.Flags().Bool(flagReset, false, "Optional flag to regenerate the enclave registration key")
	cmd.Flags().String(flagRegistrationService, "http://register.mainnet.enigma.co:36667", "Endpoint for registration service")
	cmd.Flags().String(flagBootstrapNode, "http://node1.supernova.enigma.co:1317", "REST API endpoint of a current node or light client service")

	return cmd
}

func removeQuotes(s string) string {
	return strings.Trim(s, "\"")
}
