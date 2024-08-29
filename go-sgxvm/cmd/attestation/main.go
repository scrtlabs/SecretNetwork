package main

import "github.com/SigmaGmbH/librustgo/cmd/attestation/cmd"

func main() {
	if err := cmd.RootCmd().Execute(); err != nil {
		panic(err)
	}
}
