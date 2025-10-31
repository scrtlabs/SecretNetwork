//go:build secretcli
// +build secretcli

package main

import (
	"github.com/spf13/cobra"
)

const flagReset = "reset"

func InitAttestation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init-enclave [output-file]",
		Short: "Perform remote attestation of the enclave",
		Long: `Create attestation report, signed by Intel which is used in the registration process of
the node to the chain. This process, if successful, will output a certificate which is used to authenticate with the 
blockchain. Writes the certificate in DER format to ~/attestation_cert
`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			println("This is a secretd only function, yo")
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
			println("This is a secretd only function, yo")
			return nil
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
			println("This is a secretd only function, yo")
			return nil
		},
	}

	return cmd
}

func DumpBin() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump [binary file]",
		Short: "Dump a binary file",
		Long: "Helper to display the contents of a binary file, and extract the public key of the secret node, which is used to" +
			"register the node, during node initialization",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			println("This is a secretd only function, yo")
			return nil
		},
	}

	return cmd
}

func MigrationOp() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate_op [opcode]",
		Short: "Migration operation",
		Long:  "0: migrate from SGX 2.17 format, 1: create migration report, 2: export sealing key for the new enclave, 3: import sealing data, 4: import legacy data, 5: self target info",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			println("This is a secretd only function, yo")
			return nil
		},
	}

	return cmd
}

func EmergencyApproveUpgrade() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "emergency_approve_upgrade",
		Short: "Emergency enclave upgrade approval",
		Long:  "Approve enclave upgrade in an offline mode. Need to reach consensus among network validators",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			println("This is a secretd only function, yo")
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
			println("This is a secretd only function, yo")
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
			println("This is a secretd only function, yo")
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
			println("This is a secretd only function, yo")
			return nil
		},
	}

	return cmd
}

func AutoRegisterNode() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auto-register",
		Short: "Perform remote attestation of the enclave",
		Long: `Automatically handles all registration processes. ***EXPERIMENTAL***
Please report any issues with this command
`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			println("This is a secretd only function, yo")
			return nil
		},
	}

	return cmd
}
