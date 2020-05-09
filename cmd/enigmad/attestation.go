package main

import (
	"fmt"
	"github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/api"

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
