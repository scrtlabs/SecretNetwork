package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/api"
	reg "github.com/enigmampc/EnigmaBlockchain/x/registration"
	ra "github.com/enigmampc/EnigmaBlockchain/x/registration/remote_attestation"
	"io/ioutil"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/spf13/cobra"
)

func InitAttestation(
	_ *server.Context, _ *codec.Codec) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "init-enclave [output-file]",
		Short: "Perform remote attestation of the enclave",
		Long: `Create attestation report, signed by Intel which is used in the registation process of
the node to the chain. This process, if successful, will output a certificate which is used to authenticate with the 
blockchain. Writes the certificate in DER format to ~/attestation_cert.der
`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {

			_, err := api.CreateAttestationReport()
			if err != nil {
				return fmt.Errorf("failed to create attestation report: %w", err)
			}
			return nil
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
		Use:   "configure [cert file] [seed]",
		Short: "After registration is successful, configure the secret node with the credentials file",
		Args:  cobra.ExactArgs(2),
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

			// parse coins trying to be sent
			seed := args[1]

			// fmt.Println(fmt.Sprintf("0x%s", hex.EncodeToString(pubkey)))

			cfg := reg.SeedConfig{
				EncryptedKey: seed,
				PublicKey:    hex.EncodeToString(pubkey),
			}

			cfgBytes, err := json.Marshal(&cfg)
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(reg.SecretNodeSeedConfig, cfgBytes, 0644)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}
