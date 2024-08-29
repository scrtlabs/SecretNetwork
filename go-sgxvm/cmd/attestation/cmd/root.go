package cmd

import (
	"fmt"
	"github.com/SigmaGmbH/librustgo/internal/api"
	"github.com/spf13/cobra"
	"strconv"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attestation-server",
		Short: "Commands for interaction with Swisstronik Attestation Server",
	}

	cmd.AddCommand(
		StartAttestationServer(),
		AddNewEpoch(),
		ListEpochs(),
		RemoveLatestEpoch(),
	)

	return cmd
}

// StartAttestationServer returns start-attestation-server cobra Command.
func StartAttestationServer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-server [epid-address-with-port] [dcap-address-with-port]",
		Short: "Starts attestation server",
		Long:  "Start server for Intel SGX Remote Attestation to share encryption keys with new nodes",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := api.StartAttestationServer(args[0], args[1]); err != nil {
				return err
			}
			return WaitForQuitSignals()
		},
	}

	return cmd
}

// AddNewEpoch returns create-epoch-key cobra Command.
func AddNewEpoch() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-new-epoch [starting-block]",
		Short: "Creates new epoch",
		Long:  "Creates new epoch inside Intel SGX Enclave",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			startingBlock, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				panic(err)
			}

			if err := api.AddEpoch(startingBlock); err != nil {
				panic(err)
			}
		},
	}

	return cmd
}

// ListEpochs returns list-epochs cobra Command.
func ListEpochs() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-epochs",
		Short: "Lists all stored epochs",
		Long:  "Lists all stored epochs with their starting blocks",
		Run: func(_ *cobra.Command, args []string) {
			res, err := api.ListEpochs()

			if err != nil {
				panic(err)
			}

			for _, epoch := range res {
				fmt.Println("Epoch #", epoch.EpochNumber, "Starting block: ", epoch.StartingBlock)
			}
		},
	}

	return cmd
}

// RemoveLatestEpoch returns remove-epoch cobra Command.
func RemoveLatestEpoch() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-latest-epoch",
		Short: "Removes latest epoch ",
		Long:  "Allows to remove latest epoch, for example in case, if epoch starting block was set incorrectly",
		Run: func(_ *cobra.Command, args []string) {
			if err := api.RemoveLatestEpoch(); err != nil {
				panic(err)
			}
		},
	}

	return cmd
}
