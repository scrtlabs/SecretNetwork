package main

import (
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	"github.com/scrtlabs/SecretNetwork/app"
	"os"
)

func main() {
	rootCmd, _ := NewRootCmd()
	if err := svrcmd.Execute(rootCmd, "SECRET_NETWORK", app.DefaultNodeHome); err != nil {
		os.Exit(1)
	}
}
