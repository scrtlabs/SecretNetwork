package main

import (
	"fmt"
	"os"

	wasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm"
)

// This is just a demo to ensure we can compile a static go binary
func main() {
	file := os.Args[1]
	fmt.Printf("Running %s...\n", file)
	bz, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}
	fmt.Println("Loaded!")

	err = os.MkdirAll("tmp", 0o755)
	if err != nil {
		panic(err)
	}

	wasmer, err := wasm.NewWasmer("tmp", "staking,stargate,ibc3", 0, 15)
	if err != nil {
		panic(err)
	}

	random := wasm.GetRandomNumber()
	fmt.Println("", random)

	id, err := wasmer.Create(bz)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Got code id: %X\n", id)

	wasmer.Cleanup()
	fmt.Println("finished")
}
